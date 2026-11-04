package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrMovieNotFound = errors.New("movie not found")

type Movie struct {
	ID          int64
	Title       string
	ReleaseYear int32
	Description *string
}

type MovieRating struct {
	ID      int64
	Title   string
	Count   int64
	Average *float64
}

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) AddMovie(ctx context.Context, title string, year int32) (Movie, error) {
	var movie Movie
	err := r.pool.QueryRow(ctx, `
		INSERT INTO movies (title, release_year)
		VALUES ($1, $2)
		RETURNING id, title, release_year, description`, title, year,
	).Scan(&movie.ID, &movie.Title, &movie.ReleaseYear, &movie.Description)
	if err != nil {
		return Movie{}, fmt.Errorf("add movie: %w", err)
	}
	return movie, nil
}

func (r *Repository) GetMovie(ctx context.Context, id int64) (Movie, error) {
	var movie Movie
	err := r.pool.QueryRow(ctx, `
		SELECT id, title, release_year, description
		FROM movies WHERE id = $1`, id,
	).Scan(&movie.ID, &movie.Title, &movie.ReleaseYear, &movie.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		return Movie{}, fmt.Errorf("get movie %d: %w", id, ErrMovieNotFound)
	}
	if err != nil {
		return Movie{}, fmt.Errorf("get movie %d: %w", id, err)
	}
	return movie, nil
}

func (r *Repository) EnsureUser(ctx context.Context, name, email string) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO users (name, email) VALUES ($1, $2)
		ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, name, email,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("ensure user: %w", err)
	}
	return id, nil
}

func (r *Repository) RateMovie(ctx context.Context, userID, movieID int64, score int32) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO ratings (user_id, movie_id, score) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, movie_id) DO UPDATE SET score = EXCLUDED.score`,
		userID, movieID, score,
	)
	if err != nil {
		return fmt.Errorf("rate movie: %w", err)
	}
	return nil
}

func (r *Repository) ListRatings(ctx context.Context) ([]MovieRating, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.title, COUNT(r.score), AVG(r.score)::double precision
		FROM movies AS m
		LEFT JOIN ratings AS r ON r.movie_id = m.id
		GROUP BY m.id, m.title
		ORDER BY m.id`)
	if err != nil {
		return nil, fmt.Errorf("list ratings: %w", err)
	}
	defer rows.Close()

	result := make([]MovieRating, 0)
	for rows.Next() {
		var movie MovieRating
		if err := rows.Scan(&movie.ID, &movie.Title, &movie.Count, &movie.Average); err != nil {
			return nil, fmt.Errorf("scan rating: %w", err)
		}
		result = append(result, movie)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ratings: %w", err)
	}
	return result, nil
}
