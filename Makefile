.PHONY: up down build sqlc run test vet

up:
	docker compose up --build -d

down:
	docker compose down

build:
	go build ./...

sqlc:
	docker run --rm -v $(PWD):/src -w /src sqlc/sqlc generate

run:
	go run ./cmd/server

test:
	go test ./...

vet:
	go vet ./...
