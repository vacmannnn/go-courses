package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/vacmannnn/go-courses/lecture7_sql/internal/catalog"
	"github.com/vacmannnn/go-courses/lecture7_sql/internal/db"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	id := flag.Int64("id", 0, "read an existing movie instead of creating demo data")
	list := flag.Bool("list", false, "show stored movies and their average ratings")
	flag.Parse()
	if *id < 0 || (*id != 0 && *list) {
		return errors.New("use either a positive -id or -list")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://lecture:lecture@localhost:5437/movies?sslmode=disable"
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}
	defer pool.Close()

	pingCtx, cancelPing := context.WithTimeout(ctx, 3*time.Second)
	err = pool.Ping(pingCtx)
	cancelPing()
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	requestCtx, cancelRequest := context.WithTimeout(ctx, 5*time.Second)
	defer cancelRequest()
	repository := catalog.New(pool)
	if *id != 0 {
		return showMovie(requestCtx, pool, repository, *id)
	}
	if !*list {
		if err := createDemo(requestCtx, pool, repository); err != nil {
			return err
		}
	}
	return showRatings(requestCtx, repository)
}

func createDemo(ctx context.Context, pool *pgxpool.Pool, repository *catalog.Repository) error {
	movie, err := repository.AddMovie(ctx, "Начало", 2010)
	if err != nil {
		return err
	}
	unrated, err := repository.AddMovie(ctx, "Интерстеллар", 2014)
	if err != nil {
		return err
	}
	fmt.Printf("created movie=%d and unrated movie=%d\n", movie.ID, unrated.ID)
	if err := showMovie(ctx, pool, repository, movie.ID); err != nil {
		return err
	}

	anna, err := repository.EnsureUser(ctx, "Анна", "anna@example.com")
	if err != nil {
		return err
	}
	alexander, err := repository.EnsureUser(ctx, "Александр", "alexander@example.com")
	if err != nil {
		return err
	}
	for _, rating := range []struct {
		userID int64
		score  int32
	}{{anna, 7}, {anna, 9}, {alexander, 8}} {
		if err := repository.RateMovie(ctx, rating.userID, movie.ID, rating.score); err != nil {
			return err
		}
	}

	// Проверку диапазона выполняет CHECK в БД, в том числе при ON CONFLICT.
	err = repository.RateMovie(ctx, anna, movie.ID, 11)
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) || postgresError.Code != "23514" {
		return fmt.Errorf("expected CHECK violation, got %v", err)
	}
	fmt.Println("score=11 rejected by CHECK, SQLSTATE=23514")

	_, err = repository.GetMovie(ctx, 0)
	if !errors.Is(err, catalog.ErrMovieNotFound) {
		return fmt.Errorf("expected missing movie, got %v", err)
	}
	fmt.Println("movie=0 not found, distinct from a NULL description")
	return nil
}

func showMovie(ctx context.Context, pool *pgxpool.Pool, repository *catalog.Repository, id int64) error {
	movie, err := repository.GetMovie(ctx, id)
	if err != nil {
		return err
	}
	description := "NULL"
	if movie.Description != nil {
		description = *movie.Description
	}
	fmt.Printf("manual movie=%d title=%q year=%d description=%s\n", movie.ID, movie.Title, movie.ReleaseYear, description)

	// sqlc сгенерировал Scan и тип результата из queries/movies.sql.
	generated, err := db.New(pool).GetMovie(ctx, id)
	if err != nil {
		return fmt.Errorf("sqlc get movie: %w", err)
	}
	fmt.Printf("sqlc   movie=%d title=%q year=%d description_valid=%t\n", generated.ID, generated.Title, generated.ReleaseYear, generated.Description.Valid)
	return nil
}

func showRatings(ctx context.Context, repository *catalog.Repository) error {
	movies, err := repository.ListRatings(ctx)
	if err != nil {
		return err
	}
	for _, movie := range movies {
		average := "NULL"
		if movie.Average != nil {
			average = fmt.Sprintf("%.1f", *movie.Average)
		}
		fmt.Printf("movie=%d title=%q ratings=%d average=%s\n", movie.ID, movie.Title, movie.Count, average)
	}
	return nil
}
