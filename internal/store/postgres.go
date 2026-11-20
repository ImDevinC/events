package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	"events/internal/event"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/001_events.sql
var migrationFiles embed.FS

// Postgres stores events in a PostgreSQL database.
type Postgres struct {
	db *sql.DB // db is the PostgreSQL connection pool.
}

// Open connects to PostgreSQL and checks the connection.
func Open(ctx context.Context, databaseURL string, maxOpenConnections int, maxIdleConnections int, connectionLifetime time.Duration) (*Postgres, error) {
	if maxOpenConnections < 1 || maxIdleConnections < 0 || maxIdleConnections > maxOpenConnections {
		return nil, errors.New("invalid PostgreSQL connection pool limits")
	}
	if connectionLifetime <= 0 {
		return nil, errors.New("PostgreSQL connection lifetime must be greater than zero")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL connection: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConnections)
	db.SetMaxIdleConns(maxIdleConnections)
	db.SetConnMaxLifetime(connectionLifetime)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	return &Postgres{db: db}, nil
}

// Close closes the database connection pool.
func (p *Postgres) Close() error {
	return p.db.Close()
}

// Ping checks that PostgreSQL is available.
func (p *Postgres) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

// Migrate creates or updates the event tables.
func (p *Postgres) Migrate(ctx context.Context) error {
	query, err := migrationFiles.ReadFile("migrations/001_events.sql")
	if err != nil {
		return fmt.Errorf("read database migration: %w", err)
	}
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin database migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('events:migrations'))`); err != nil {
		return fmt.Errorf("lock database migrations: %w", err)
	}
	if _, err := tx.ExecContext(ctx, string(query)); err != nil {
		return fmt.Errorf("run database migration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database migration: %w", err)
	}
	return nil
}

// Create inserts an event and returns the saved event.
func (p *Postgres) Create(ctx context.Context, item event.Event) (event.Event, error) {
	return scanEvent(p.db.QueryRowContext(ctx, `
		INSERT INTO events (name, event_date, event_time, cost_cents, link, image_url, tickets_required, tickets_purchased)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, name, event_date, event_time, cost_cents, link, image_url, tickets_required, tickets_purchased`,
		item.Name, item.Date, item.Time, nullableCost(item.CostCents), item.Link, item.ImageURL, item.TicketsRequired, item.TicketsPurchased))
}

// Get returns one event by its ID.
func (p *Postgres) Get(ctx context.Context, id int64) (event.Event, error) {
	return scanEvent(p.db.QueryRowContext(ctx, `
		SELECT id, name, event_date, event_time, cost_cents, link, image_url, tickets_required, tickets_purchased
		FROM events WHERE id = $1`, id))
}

// Update changes a saved event and returns the updated event.
func (p *Postgres) Update(ctx context.Context, item event.Event) (event.Event, error) {
	saved, err := scanEvent(p.db.QueryRowContext(ctx, `
		UPDATE events
		SET name = $2, event_date = $3, event_time = $4, cost_cents = $5,
		    link = $6, image_url = $7, tickets_required = $8, tickets_purchased = $9, updated_at = now()
		WHERE id = $1
		RETURNING id, name, event_date, event_time, cost_cents, link, image_url, tickets_required, tickets_purchased`,
		item.ID, item.Name, item.Date, item.Time, nullableCost(item.CostCents), item.Link, item.ImageURL, item.TicketsRequired, item.TicketsPurchased))
	return saved, err
}

// Upcoming returns events on or after the given date.
func (p *Postgres) Upcoming(ctx context.Context, from time.Time) ([]event.Event, error) {
	return p.list(ctx, `
		SELECT id, name, event_date, event_time, cost_cents, link, image_url, tickets_required, tickets_purchased
		FROM events WHERE event_date >= $1
		ORDER BY event_date, event_time NULLS LAST, id`, from)
}

// Between returns events on or after start and before end.
func (p *Postgres) Between(ctx context.Context, start time.Time, end time.Time) ([]event.Event, error) {
	return p.list(ctx, `
		SELECT id, name, event_date, event_time, cost_cents, link, image_url, tickets_required, tickets_purchased
		FROM events WHERE event_date >= $1 AND event_date < $2
		ORDER BY event_date, event_time NULLS LAST, id`, start, end)
}

func (p *Postgres) list(ctx context.Context, query string, arguments ...any) ([]event.Event, error) {
	rows, err := p.db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()
	items := make([]event.Event, 0)
	for rows.Next() {
		item, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}
	return items, nil
}

// rowScanner reads values from a database row.
type rowScanner interface {
	// Scan copies the columns from the current row into dest.
	Scan(dest ...any) error
}

func scanEvent(row rowScanner) (event.Event, error) {
	var item event.Event
	var eventDate time.Time
	var eventTime string
	var cost sql.NullInt64
	var link string
	var imageURL string
	if err := row.Scan(&item.ID, &item.Name, &eventDate, &eventTime, &cost, &link, &imageURL, &item.TicketsRequired, &item.TicketsPurchased); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return event.Event{}, sql.ErrNoRows
		}
		return event.Event{}, fmt.Errorf("scan event: %w", err)
	}
	item.Date = eventDate
	item.Time = eventTime
	item.Link = link
	item.ImageURL = imageURL
	if cost.Valid {
		item.CostCents = &cost.Int64
	}
	return item, nil
}

func nullableCost(cost *int64) any {
	if cost == nil {
		return nil
	}
	return *cost
}
