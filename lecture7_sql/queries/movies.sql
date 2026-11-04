-- name: GetMovie :one
SELECT id, title, release_year, description
FROM movies
WHERE id = $1;

-- name: ListMovies :many
SELECT id, title, release_year, description
FROM movies
ORDER BY id;
