-- +goose Up
CREATE TABLE movies (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title text NOT NULL,
    release_year integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL,
    email text NOT NULL UNIQUE
);

CREATE TABLE ratings (
    user_id bigint REFERENCES users(id),
    movie_id bigint REFERENCES movies(id),
    score integer NOT NULL CHECK (score BETWEEN 1 AND 10),
    PRIMARY KEY (user_id, movie_id)
);

-- +goose Down
DROP TABLE ratings;
DROP TABLE users;
DROP TABLE movies;
