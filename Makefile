.PHONY: run test docker-build fmt

run:
	DATABASE_URL='postgres://events:password@localhost:5432/events?sslmode=disable' go run ./cmd/events

test:
	./scripts/test.sh

docker-build:
	docker build -t events:local .

fmt:
	gofmt -w cmd internal

docker-postgres:
	docker run -d --rm --name events-postgres -e POSTGRES_USER=events -e POSTGRES_PASSWORD=password -e POSTGRES_DB=events -p 5432:5432 postgres:18-alpine
