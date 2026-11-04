package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vacmannnn/go-courses/lecture7_sql/internal/db"
)

func TestRepositoryIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL and apply migrations to run the PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repository := New(pool)

	movie, err := repository.AddMovie(ctx, "integration", 2010)
	if err != nil {
		t.Fatal(err)
	}
	var userID int64
	defer func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if _, err := pool.Exec(cleanupCtx, "DELETE FROM ratings WHERE movie_id = $1", movie.ID); err != nil {
			t.Error(err)
		}
		if _, err := pool.Exec(cleanupCtx, "DELETE FROM users WHERE id = $1", userID); err != nil {
			t.Error(err)
		}
		if _, err := pool.Exec(cleanupCtx, "DELETE FROM movies WHERE id = $1", movie.ID); err != nil {
			t.Error(err)
		}
	}()
	email := fmt.Sprintf("lecture7-%d@example.com", movie.ID)
	userID, err = repository.EnsureUser(ctx, "Test", email)
	if err != nil {
		t.Fatal(err)
	}

	got, err := repository.GetMovie(ctx, movie.ID)
	if err != nil || got.Description != nil {
		t.Fatalf("movie with NULL description: %+v, %v", got, err)
	}
	generated, err := db.New(pool).GetMovie(ctx, movie.ID)
	if err != nil || generated.ID != got.ID || generated.Description.Valid {
		t.Fatalf("sqlc result: %+v, %v", generated, err)
	}
	if _, err := repository.GetMovie(ctx, 0); !errors.Is(err, ErrMovieNotFound) {
		t.Fatalf("missing movie: %v", err)
	}
	unrated, err := repository.ListRatings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	unratedFound := false
	for _, rating := range unrated {
		if rating.ID == movie.ID {
			unratedFound = true
			if rating.Count != 0 || rating.Average != nil {
				t.Fatalf("unrated movie should have count 0 and NULL average: %+v", rating)
			}
		}
	}
	if !unratedFound {
		t.Fatal("LEFT JOIN dropped an unrated movie")
	}
	if err := repository.RateMovie(ctx, userID, movie.ID, 7); err != nil {
		t.Fatal(err)
	}
	if err := repository.RateMovie(ctx, userID, movie.ID, 9); err != nil {
		t.Fatal(err)
	}
	err = repository.RateMovie(ctx, userID, movie.ID, 11)
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != "23514" {
		t.Fatalf("expected CHECK violation: %v", err)
	}
	results, err := repository.ListRatings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, rating := range results {
		if rating.ID == movie.ID {
			found = true
			if rating.Count != 1 || rating.Average == nil || *rating.Average != 9 {
				t.Fatalf("upsert and rejected score changed ratings: %+v", rating)
			}
		}
	}
	if !found {
		t.Fatal("created movie missing from ratings")
	}
}
