#!/bin/sh
set -eu

export TEST_POSTGRES_PORT="${TEST_POSTGRES_PORT:-55432}"
export TEST_DATABASE_URL="${TEST_DATABASE_URL:-postgres://events:events@127.0.0.1:${TEST_POSTGRES_PORT}/events_test?sslmode=disable}"

container_name="events-test-postgres-$$"

cleanup() {
	if docker container inspect "$container_name" >/dev/null 2>&1; then
		docker stop "$container_name" >/dev/null
	fi
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

docker run --detach --rm \
	--name "$container_name" \
	--publish "127.0.0.1:${TEST_POSTGRES_PORT}:5432" \
	--env POSTGRES_DB=events_test \
	--env POSTGRES_USER=events \
	--env POSTGRES_PASSWORD=events \
	--health-cmd='pg_isready -U events -d events_test' \
	--health-interval=2s \
	--health-timeout=3s \
	--health-retries=20 \
	--health-start-period=2s \
	postgres:17-alpine >/dev/null

attempt=0
until [ "$(docker inspect --format '{{.State.Health.Status}}' "$container_name" 2>/dev/null || true)" = "healthy" ]; do
	attempt=$((attempt + 1))
	if [ "$attempt" -ge 60 ]; then
		docker logs "$container_name"
		exit 1
	fi
	sleep 1
done

go test ./...
