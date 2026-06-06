.PHONY: test test-race test-integration lint fmt migrate-up migrate-down run-api run-outbox run-reconciler

test:
	go test ./...

test-race:
	go test -race ./...

test-integration:
	go test -tags=integration ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .

migrate-up:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down 1

run-api:
	go run ./cmd/api

run-outbox:
	go run ./cmd/outbox-worker

run-reconciler:
	go run ./cmd/reconciler