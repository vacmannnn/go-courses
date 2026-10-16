.PHONY: generate server client test check

generate:
	mkdir -p gen/moviesv1
	protoc -I proto \
		--go_out=gen/moviesv1 --go_opt=paths=source_relative \
		--go-grpc_out=gen/moviesv1 --go-grpc_opt=paths=source_relative \
		movie.proto

server:
	go run ./cmd/server

client:
	go run ./cmd/client

test:
	go test -race ./...

check: test
	go vet ./...
