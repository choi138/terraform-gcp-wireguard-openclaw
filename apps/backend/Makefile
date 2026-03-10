.PHONY: fmt test run-api migrate

fmt:
	gofmt -w ./cmd ./internal

test:
	go test ./...

run-api:
	go run ./cmd/api

migrate:
	go run ./cmd/migrate
