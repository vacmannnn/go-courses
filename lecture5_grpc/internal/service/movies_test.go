package service

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	pb "example.com/grpcdemo/gen/moviesv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// Real gRPC serialization and status handling, without using a public TCP port.
func testClient(t *testing.T, unaryDelay, streamDelay time.Duration, opts ...grpc.ServerOption) pb.MovieServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer(opts...)
	pb.RegisterMovieServiceServer(srv, NewMovieServer(unaryDelay, streamDelay))
	go func() { _ = srv.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(); srv.Stop(); lis.Close() })
	return pb.NewMovieServiceClient(conn)
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestUnaryResults(t *testing.T) {
	client := testClient(t, 0, 0)
	for _, tc := range []struct {
		id   int64
		code codes.Code
	}{{42, codes.OK}, {0, codes.InvalidArgument}, {-1, codes.InvalidArgument}, {999, codes.NotFound}} {
		m, err := client.GetMovie(testContext(t), &pb.GetMovieRequest{Id: tc.id})
		if status.Code(err) != tc.code {
			t.Fatalf("id=%d: got %v, want %v", tc.id, err, tc.code)
		}
		if tc.code == codes.OK && (m.GetId() != 42 || m.GetTitle() != "Inception" || m.GetYear() != 2010) {
			t.Fatalf("unexpected movie: %v", m)
		}
	}
}

func TestDeadlineStopsServerWork(t *testing.T) {
	done := make(chan error, 1)
	client := testClient(t, time.Second, 0, grpc.UnaryInterceptor(
		func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			resp, err := handler(ctx, req)
			if req.(*pb.GetMovieRequest).GetId() == 42 {
				done <- err
			}
			return resp, err
		}))
	// Establish the channel before measuring the deadline of the slow handler.
	_, _ = client.GetMovie(testContext(t), &pb.GetMovieRequest{Id: 0})
	ctx, cancel := context.WithTimeout(testContext(t), 50*time.Millisecond)
	defer cancel()
	_, err := client.GetMovie(ctx, &pb.GetMovieRequest{Id: 42})
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("got %v, want DeadlineExceeded", err)
	}
	select {
	case err := <-done:
		if code := status.Code(err); code != codes.Canceled && code != codes.DeadlineExceeded {
			t.Fatalf("server did not observe cancellation: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("server continued its one-second delay after the client timed out")
	}
}

func TestStreamingOrderAndEOF(t *testing.T) {
	client := testClient(t, 0, 0)
	stream, err := client.LoadMoviesStream(testContext(t), &pb.LoadMoviesRequest{From: 1, To: 5})
	if err != nil {
		t.Fatal(err)
	}
	for id := int64(1); id <= 5; id++ {
		p, err := stream.Recv()
		if err != nil || p.GetMovieId() != id || p.GetLoaded() != int32(id) {
			t.Fatalf("message %d: progress=%v err=%v", id, p, err)
		}
	}
	if _, err := stream.Recv(); err != io.EOF {
		t.Fatalf("got %v, want EOF", err)
	}
}

func TestStreamingPartialFailure(t *testing.T) {
	client := testClient(t, 0, 0)
	stream, err := client.LoadMoviesStream(testContext(t), &pb.LoadMoviesRequest{From: 4, To: 6})
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []int64{4, 5} {
		p, err := stream.Recv()
		if err != nil || p.GetMovieId() != id || p.GetLoaded() != int32(i+1) {
			t.Fatalf("progress=%v err=%v", p, err)
		}
	}
	if _, err := stream.Recv(); status.Code(err) != codes.NotFound {
		t.Fatalf("got %v, want NotFound after two messages", err)
	}
}

func TestStreamingInvalidRange(t *testing.T) {
	client := testClient(t, 0, 0)
	for _, req := range []*pb.LoadMoviesRequest{{From: 0, To: 5}, {From: 5, To: 1}, {From: 1, To: 101}} {
		stream, err := client.LoadMoviesStream(testContext(t), req)
		if err == nil {
			_, err = stream.Recv()
		}
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("range=%v: got %v, want InvalidArgument", req, err)
		}
	}
}

func TestStreamingCancellationStopsServer(t *testing.T) {
	done := make(chan error, 1)
	client := testClient(t, 0, 100*time.Millisecond, grpc.StreamInterceptor(
		func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			err := handler(srv, stream)
			done <- err
			return err
		}))
	ctx, cancel := context.WithCancel(testContext(t))
	defer cancel()
	stream, err := client.LoadMoviesStream(ctx, &pb.LoadMoviesRequest{From: 1, To: 5})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatal(err)
	}
	cancel()
	for {
		_, err = stream.Recv()
		if err != nil {
			break
		}
	}
	if status.Code(err) != codes.Canceled {
		t.Fatalf("got %v, want Canceled", err)
	}
	select {
	case err := <-done:
		if status.Code(err) != codes.Canceled {
			t.Fatalf("server got %v, want Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop after cancellation")
	}
}

func TestUnaryInterceptorLogsMetadataAndStatus(t *testing.T) {
	var output bytes.Buffer
	logger := log.New(&output, "", 0)
	client := testClient(t, 0, 0, grpc.UnaryInterceptor(UnaryLogging(logger)))
	ctx := metadata.AppendToOutgoingContext(testContext(t), "x-request-id", "lecture-42")
	for _, id := range []int64{42, 0} {
		_, _ = client.GetMovie(ctx, &pb.GetMovieRequest{Id: id})
	}
	for _, text := range []string{"/movies.v1.MovieService/GetMovie", `request_id="lecture-42"`, "code=OK", "code=InvalidArgument", "duration="} {
		if !strings.Contains(output.String(), text) {
			t.Fatalf("missing %q in logs: %s", text, output.String())
		}
	}
}
