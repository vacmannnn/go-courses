-- name: AddMovie :one
INSERT INTO movies (title, release_year)
VALUES ($1, $2)
RETURNING id;

-- name: GetMovie :one
SELECT id, title, release_year FROM movies WHERE id = $1;

-- name: RenameMovie :exec
UPDATE movies SET title = $1 WHERE id = $2;

-- name: ListMovies :many
SELECT id, title, release_year FROM movies ORDER BY id;
