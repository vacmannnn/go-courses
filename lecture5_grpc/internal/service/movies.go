// Package service implements the handwritten part of MovieService.
package service

import (
	"context"
	"time"

	pb "example.com/grpcdemo/gen/moviesv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// These domain values are read-only. Handlers create a fresh protobuf response.
type movie struct {
	id    int64
	title string
	year  int32
}

type MovieServer struct {
	pb.UnimplementedMovieServiceServer
	catalog     map[int64]movie
	unaryDelay  time.Duration
	streamDelay time.Duration
}

func NewMovieServer(unaryDelay, streamDelay time.Duration) *MovieServer {
	return &MovieServer{
		unaryDelay: unaryDelay, streamDelay: streamDelay,
		catalog: map[int64]movie{
			1:  {1, "The Matrix", 1999},
			2:  {2, "Interstellar", 2014},
			3:  {3, "Arrival", 2016},
			4:  {4, "Spirited Away", 2001},
			5:  {5, "The Truman Show", 1998},
			42: {42, "Inception", 2010},
		},
	}
}

func (s *MovieServer) GetMovie(ctx context.Context, req *pb.GetMovieRequest) (*pb.Movie, error) {
	if req.GetId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "id must be positive")
	}
	m, ok := s.catalog[req.GetId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "movie not found")
	}
	if err := wait(ctx, s.unaryDelay); err != nil {
		return nil, err
	}
	return &pb.Movie{Id: m.id, Title: m.title, Year: m.year}, nil
}

func (s *MovieServer) LoadMoviesStream(req *pb.LoadMoviesRequest, stream grpc.ServerStreamingServer[pb.LoadProgress]) error {
	from, to := req.GetFrom(), req.GetTo()
	if from <= 0 || to < from || to-from >= 100 {
		return status.Error(codes.InvalidArgument, "range must be positive, ordered and contain at most 100 movies")
	}
	var loaded int32
	for id := from; ; id++ {
		if err := s.loadMovie(stream.Context(), id); err != nil {
			return err
		}
		loaded++
		if err := stream.Send(&pb.LoadProgress{MovieId: id, Loaded: loaded}); err != nil {
			return err
		}
		// Check before incrementing: even to=MaxInt64 cannot overflow the loop.
		if id == to {
			return nil
		}
	}
}

func (s *MovieServer) loadMovie(ctx context.Context, id int64) error {
	if _, ok := s.catalog[id]; !ok {
		return status.Error(codes.NotFound, "movie not found")
	}
	// Instead of an external film service, emulate its latency.
	return wait(ctx, s.streamDelay)
}

func wait(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return status.FromContextError(err).Err()
	}
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return status.FromContextError(ctx.Err()).Err()
	}
}
