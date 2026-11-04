-- +goose Up
ALTER TABLE movies ADD COLUMN description text;

-- +goose Down
ALTER TABLE movies DROP COLUMN description;
