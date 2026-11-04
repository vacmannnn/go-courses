package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/vacmannnn/go-courses/lecture7/internal/db"
	"github.com/vacmannnn/go-courses/lecture7/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://lecture:lecture@localhost:5437/movies?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// 0. Накатываем встроенные SQL-миграции до первого запроса.
	// Goose работает через database/sql, поэтому адаптируем пул pgx.
	migrationDB := stdlib.OpenDBFromPool(pool)
	defer migrationDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, migrationDB, migrations.Files)
	if err != nil {
		return fmt.Errorf("создать goose: %w", err)
	}
	// Up применяет только новые миграции, записывая версии в goose_db_version.
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("применить миграции: %w", err)
	}

	queries := db.New(pool)

	// 1. Добавляем фильм. sqlc сгенерировал метод и тип параметров из SQL.
	id, err := queries.AddMovie(ctx, db.AddMovieParams{
		Title:       "Начало",
		ReleaseYear: 2010,
	})
	if err != nil {
		return err
	}

	// 2. Читаем добавленный фильм. Результат уже имеет поля нужных типов.
	movie, err := queries.GetMovie(ctx, id)
	if err != nil {
		return err
	}
	fmt.Printf("Добавлен фильм %d: %s (%d)\n", movie.ID, movie.Title, movie.ReleaseYear)

	// 3. Меняем название.
	err = queries.RenameMovie(ctx, db.RenameMovieParams{
		Title: "Начало / Inception",
		ID:    id,
	})
	if err != nil {
		return err
	}

	// 4. Читаем список. Метод возвращает готовый срез фильмов.
	movies, err := queries.ListMovies(ctx)
	if err != nil {
		return err
	}
	for _, movie := range movies {
		fmt.Printf("%d: %s (%d)\n", movie.ID, movie.Title, movie.ReleaseYear)
	}
	return nil
}
