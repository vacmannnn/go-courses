package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
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

	// 1. Добавляем фильм. QueryRow возвращает одну строку, Scan читает ID.
	var id int64
	err = pool.QueryRow(ctx, `
		INSERT INTO movies (title, release_year) VALUES ($1, $2)
		RETURNING id`, "Начало", 2010).Scan(&id)
	if err != nil {
		return err
	}

	// 2. Читаем добавленный фильм. Значения передаём отдельно от SQL.
	var title string
	var year int32
	err = pool.QueryRow(ctx, `
		SELECT title, release_year FROM movies WHERE id = $1`, id).Scan(&title, &year)
	if err != nil {
		return err
	}
	fmt.Printf("Добавлен фильм %d: %s (%d)\n", id, title, year)

	// 3. Меняем название. Exec выполняет запрос без возвращаемых строк.
	_, err = pool.Exec(ctx, `UPDATE movies SET title = $1 WHERE id = $2`, "Начало / Inception", id)
	if err != nil {
		return err
	}

	// 4. Читаем список. Query возвращает строки, обходим их через Next.
	rows, err := pool.Query(ctx, `SELECT id, title, release_year FROM movies ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := rows.Scan(&id, &title, &year); err != nil {
			return err
		}
		fmt.Printf("%d: %s (%d)\n", id, title, year)
	}
	return rows.Err()
}
