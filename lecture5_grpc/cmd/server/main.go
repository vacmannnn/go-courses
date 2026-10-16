package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "example.com/grpcdemo/gen/moviesv1"
	"example.com/grpcdemo/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:50051", "listen address")
	unaryDelay := flag.Duration("unary-delay", 0, "delay before a successful GetMovie response")
	streamDelay := flag.Duration("stream-delay", 300*time.Millisecond, "delay per streamed movie")
	flag.Parse()
	if *unaryDelay < 0 || *streamDelay < 0 {
		log.Fatal("delays must be non-negative")
	}
	logger := log.New(os.Stderr, "server ", log.Ltime|log.Lmicroseconds)
	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		logger.Fatal(err)
	}
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(service.UnaryLogging(logger)),
		grpc.StreamInterceptor(service.StreamLogging(logger)),
	)
	pb.RegisterMovieServiceServer(srv, service.NewMovieServer(*unaryDelay, *streamDelay))
	reflection.Register(srv) // Allows grpcurl to discover methods and messages.

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	go func() {
		<-stop
		logger.Println("stopping")
		// Stop after a short grace period if a stream is still running.
		timer := time.AfterFunc(2*time.Second, srv.Stop)
		srv.GracefulStop()
		timer.Stop()
	}()
	logger.Printf("listening=%s unary_delay=%s stream_delay=%s", lis.Addr(), *unaryDelay, *streamDelay)
	if err := srv.Serve(lis); err != nil {
		logger.Fatal(err)
	}
}
