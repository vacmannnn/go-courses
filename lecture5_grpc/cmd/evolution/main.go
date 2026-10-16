// This example simulates old and new clients in one process using real protobuf bytes.
package main

import (
	"fmt"
	"log"

	pb "example.com/grpcdemo/gen/moviesv1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Build the old schema from the current descriptor by removing Movie.year.
	oldFile := protodesc.ToFileDescriptorProto(pb.File_movie_proto)
	oldFile.Service = nil
	for _, message := range oldFile.MessageType {
		if message.GetName() == "Movie" {
			message.Field = message.Field[:2]
		}
	}
	file, err := protodesc.NewFile(oldFile, nil)
	if err != nil {
		return err
	}
	oldMovieType := file.Messages().ByName("Movie")
	oldMovie := dynamicpb.NewMessage(oldMovieType)
	oldMovie.Set(oldMovieType.Fields().ByNumber(1), protoreflect.ValueOfInt64(42))
	oldMovie.Set(oldMovieType.Fields().ByNumber(2), protoreflect.ValueOfString("Inception"))
	data, err := proto.Marshal(oldMovie)
	if err != nil {
		return err
	}
	var newMovie pb.Movie
	if err := proto.Unmarshal(data, &newMovie); err != nil {
		return err
	}
	fmt.Printf("Old server -> new client: id=%d title=%q year=%d\n", newMovie.GetId(), newMovie.GetTitle(), newMovie.GetYear())
	fmt.Println("The old schema has no year field; 0 is the new client's default value.")

	data, err = proto.Marshal(&pb.Movie{Id: 42, Title: "Inception", Year: 2010})
	if err != nil {
		return err
	}
	oldClientMovie := dynamicpb.NewMessage(oldMovieType)
	if err := proto.Unmarshal(data, oldClientMovie); err != nil {
		return err
	}
	fmt.Printf("New server -> old client: id=%d title=%q\n",
		oldClientMovie.Get(oldMovieType.Fields().ByNumber(1)).Int(),
		oldClientMovie.Get(oldMovieType.Fields().ByNumber(2)).String())
	fmt.Printf("year is not in the old schema; %d unknown bytes preserved.\n", len(oldClientMovie.GetUnknown()))
	return nil
}
