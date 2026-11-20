# events

`events` is a small Go and HTMX app for a list of upcoming plans. Add an event by pasting a page URL, or enter its details yourself. The calendar is a secondary view.

Event name and start and end dates are required. The end date defaults to the start date; matching dates make a single-day event, and a later end date makes an inclusive multi-day range. An end date before the start date cannot be saved. Time, cost, and link are optional. Ticket checkboxes track whether tickets are required and purchased. Both states show on the upcoming list. Multi-day events show their abbreviated date range on the Upcoming list and appear on each day in the calendar.

The appearance follows your device's light or dark mode by default. Use the sun/moon button in the header to switch modes; your choice is saved in this browser and remains in effect until you change it again.

## Build the container image

```sh
make docker-build
```

This builds the `events:local` image. The container needs a reachable PostgreSQL database. Set `DATABASE_URL` when you run it. The app applies its database migration on startup.

```sh
docker run --rm -p 8080:8080 \
  -e DATABASE_URL='postgres://events:events@postgres-host:5432/events?sslmode=disable' \
  events:local
```

Open <http://localhost:8080> after the app starts.

The application image uses a static Go build and `gcr.io/distroless/static-debian13:nonroot`. It runs as UID 65532. It has no shell or package manager.

## Run locally

Start PostgreSQL, then set `DATABASE_URL` and run:

```sh
DATABASE_URL='postgres://events:events@localhost:5432/events?sslmode=disable' go run ./cmd/events
```

The default `DATABASE_URL` matches this local connection. The app listens on `:8080` by default.

## Test

```sh
make test
```

This command starts PostgreSQL with `docker run`. It runs unit and PostgreSQL integration tests, then stops and removes the container. It stores no test data after the run.

## Configuration

All runtime settings use environment variables. The defaults below support local development.

| Variable | Default | Purpose |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | HTTP listen address |
| `DATABASE_URL` | `postgres://events:events@localhost:5432/events?sslmode=disable` | PostgreSQL connection URL |
| `APP_TIMEZONE` | `UTC` | Timezone for dates and the calendar |
| `URL_FETCH_TIMEOUT` | `8s` | Maximum duration for event page metadata requests |
| `HTTP_READ_TIMEOUT` | `10s` | Maximum duration for incoming HTTP requests |
| `HTTP_WRITE_TIMEOUT` | `20s` | Maximum duration for HTTP responses |
| `HTTP_IDLE_TIMEOUT` | `60s` | Maximum keep-alive idle duration |
| `HTTP_SHUTDOWN_TIMEOUT` | `10s` | Grace period for server shutdown |
| `DATABASE_HEALTH_TIMEOUT` | `2s` | Maximum duration for database health checks |
| `DATABASE_STARTUP_TIMEOUT` | `10s` | Maximum duration for database connection and migrations |
| `DATABASE_CONN_MAX_LIFETIME` | `30m` | Maximum age of a pooled database connection |
| `DATABASE_MAX_OPEN_CONNS` | `12` | Maximum open database connections |
| `DATABASE_MAX_IDLE_CONNS` | `4` | Maximum idle database connections |
| `TEST_POSTGRES_PORT` | `55432` | Host port for the test database |
| `TEST_DATABASE_URL` | Local Docker test URL | PostgreSQL URL used by integration tests |

## Event page import

The importer reads Open Graph and Twitter metadata, common event meta tags, and Schema.org Event JSON-LD. It fills in fields that it can find. You can review and change them before saving. A link can also load a thumbnail when you add or edit an event.

The importer only requests public HTTP and HTTPS addresses. It blocks private IP addresses, limits redirects, time, and page size, and does not run page scripts.

## Contributor documentation

Keep this README and [`AGENTS.md`](AGENTS.md) current and consistent whenever the project changes. See `AGENTS.md` for the documentation-maintenance rules and pull request impact requirements, including the matching GitHub label (`major`, `minor`, or `patch` for versioned changes, or `chore` when no version bump is required).
