package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	pb "example.com/grpcdemo/gen/moviesv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "code=%s message=%s\n", status.Code(err), status.Convert(err).Message())
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", "localhost:50051", "server address")
	mode := flag.String("mode", "get", "get or stream")
	id := flag.Int64("id", 42, "movie ID for get")
	from := flag.Int64("from", 1, "first movie ID for stream")
	to := flag.Int64("to", 5, "last movie ID for stream")
	timeout := flag.Duration("timeout", 3*time.Second, "RPC timeout")
	cancelAfter := flag.Duration("cancel-after", 0, "explicitly cancel RPC after this duration; 0 disables")
	requestID := flag.String("request-id", "demo-42", "x-request-id metadata")
	flag.Parse()
	if *timeout <= 0 || *cancelAfter < 0 {
		return fmt.Errorf("timeout must be positive; cancel-after must be non-negative")
	}
	if *mode != "get" && *mode != "stream" {
		return fmt.Errorf("unknown mode %q: use get or stream", *mode)
	}
	// TLS is disabled only for this loopback demonstration.
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	client := pb.NewMovieServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if *cancelAfter > 0 {
		timer := time.AfterFunc(*cancelAfter, cancel)
		defer timer.Stop()
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "x-request-id", *requestID)
	if *mode == "get" {
		movie, err := client.GetMovie(ctx, &pb.GetMovieRequest{Id: *id})
		if err != nil {
			return err
		}
		fmt.Printf("movie=%d title=%q year=%d\n", movie.GetId(), movie.GetTitle(), movie.GetYear())
		return nil
	}
	stream, err := client.LoadMoviesStream(ctx, &pb.LoadMoviesRequest{From: *from, To: *to})
	if err != nil {
		return err
	}
	for {
		progress, err := stream.Recv()
		if err == io.EOF {
			fmt.Println("stream completed: OK")
			return nil
		}
		if err != nil {
			return err
		}
		fmt.Printf("movie=%d loaded=%d\n", progress.GetMovieId(), progress.GetLoaded())
	}
}
