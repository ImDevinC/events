package store_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"events/internal/event"
	"events/internal/store"
)

// TestPostgresCreateReadUpdateAndList verifies event persistence in PostgreSQL.
func TestPostgresCreateReadUpdateAndList(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; run make test to start local PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	repository, err := store.Open(ctx, databaseURL, 4, 2, 30*time.Minute)
	if err != nil {
		t.Fatalf("Open() returned an error: %v", err)
	}
	defer repository.Close()
	if err := repository.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() returned an error: %v", err)
	}

	name := "Integration event " + time.Now().Format("20060102150405.000000000")
	date := time.Date(2099, time.June, 12, 0, 0, 0, 0, time.UTC)
	cost := int64(2350)
	created, err := repository.Create(ctx, event.Event{
		Name:             name,
		Date:             date,
		Time:             "19:15",
		CostCents:        &cost,
		Link:             "https://example.com/event",
		TicketsRequired:  true,
		TicketsPurchased: false,
	})
	if err != nil {
		t.Fatalf("Create() returned an error: %v", err)
	}
	if created.ID < 1 || created.Name != name || created.CostCents == nil || *created.CostCents != cost || created.Time != "19:15" || !created.TicketsRequired || created.TicketsPurchased {
		t.Fatalf("Create() returned unexpected event: %#v", created)
	}

	loaded, err := repository.Get(ctx, created.ID)
	if err != nil || loaded.Name != name || loaded.Date.Format("2006-01-02") != date.Format("2006-01-02") || !loaded.TicketsRequired || loaded.TicketsPurchased {
		t.Fatalf("Get() returned %#v, error %v", loaded, err)
	}

	loaded.Name = name + " updated"
	loaded.ImageURL = "https://example.com/poster.jpg"
	loaded.TicketsPurchased = true
	updated, err := repository.Update(ctx, loaded)
	if err != nil || updated.Name != loaded.Name || updated.ImageURL != loaded.ImageURL || !updated.TicketsRequired || !updated.TicketsPurchased {
		t.Fatalf("Update() returned %#v, error %v", updated, err)
	}

	upcoming, err := repository.Upcoming(ctx, time.Date(2099, time.June, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Upcoming() returned an error: %v", err)
	}
	found := false
	for _, item := range upcoming {
		if item.ID == created.ID && strings.HasSuffix(item.Name, "updated") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Upcoming() did not return the saved event")
	}

	between, err := repository.Between(ctx, time.Date(2099, time.June, 1, 0, 0, 0, 0, time.UTC), time.Date(2099, time.July, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Between() returned an error: %v", err)
	}
	for _, item := range between {
		if item.ID == created.ID {
			return
		}
	}
	t.Fatal("Between() did not return the saved event")
}
