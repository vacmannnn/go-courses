DATABASE_URL ?= postgres://lecture:lecture@localhost:5437/movies?sslmode=disable
GOOSE ?= goose
SQLC ?= sqlc

.PHONY: up down migrate status rollback generate run check integration

up:
	docker compose up -d --wait

down:
	docker compose down

migrate:
	$(GOOSE) -dir migrations postgres "$(DATABASE_URL)" up

status:
	$(GOOSE) -dir migrations postgres "$(DATABASE_URL)" status

rollback:
	$(GOOSE) -dir migrations postgres "$(DATABASE_URL)" down

generate:
	$(SQLC) generate

run:
	DATABASE_URL='$(DATABASE_URL)' go run ./cmd

check:
	go test -race ./...
	go vet ./...

integration:
	TEST_DATABASE_URL='$(DATABASE_URL)' go test -race -count=1 ./internal/catalog
