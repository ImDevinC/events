package web_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"events/internal/metadata"
	"events/internal/store"
	"events/internal/web"
)

// TestNewHandlerParsesTemplates verifies that the embedded web templates are valid.
func TestNewHandlerParsesTemplates(t *testing.T) {
	if _, err := web.NewHandler(nil, metadata.NewFetcher(time.Second), time.UTC, 2*time.Second); err != nil {
		t.Fatalf("NewHandler() returned an error: %v", err)
	}
}

// TestHomeShowsTicketStatusAndNonCurrentYear verifies event badges and year display on the upcoming page.
func TestHomeShowsTicketStatusAndNonCurrentYear(t *testing.T) {
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

	cleanupDatabase, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("sql.Open() returned an error: %v", err)
	}
	nameSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	currentName := "Current year test " + nameSuffix
	nextName := "Next year test " + nameSuffix
	t.Cleanup(func() {
		_, err := cleanupDatabase.ExecContext(context.Background(), `DELETE FROM events WHERE name IN ($1, $2)`, currentName, nextName)
		if err != nil {
			t.Errorf("clean up test events: %v", err)
		}
		if err := cleanupDatabase.Close(); err != nil {
			t.Errorf("close cleanup database: %v", err)
		}
	})

	currentYear := time.Now().Year()
	currentDate := time.Date(currentYear, time.December, 31, 0, 0, 0, 0, time.UTC)
	nextDate := time.Date(currentYear+1, time.January, 2, 0, 0, 0, 0, time.UTC)
	handler, err := web.NewHandler(repository, metadata.NewFetcher(time.Second), time.UTC, 2*time.Second)
	if err != nil {
		t.Fatalf("NewHandler() returned an error: %v", err)
	}
	postEvent := func(name string, date time.Time, ticketsRequired bool, ticketsPurchased bool) {
		form := url.Values{
			"name": {name},
			"date": {date.Format("2006-01-02")},
		}
		if ticketsRequired {
			form.Set("tickets_required", "true")
		}
		if ticketsPurchased {
			form.Set("tickets_purchased", "true")
		}
		request := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusSeeOther {
			t.Fatalf("POST /events returned status %d, body %q", response.Code, response.Body.String())
		}
	}
	postEvent(currentName, currentDate, true, false)
	postEvent(nextName, nextDate, true, true)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET / returned status %d", response.Code)
	}
	body := response.Body.String()
	if strings.Contains(body, currentDate.Format("January 2, 2006")) {
		t.Fatalf("current-year event date included a year: %s", currentDate.Format("January 2, 2006"))
	}
	leftDate := fmt.Sprintf(`<span class="event-month">JAN</span><span class="event-day">02</span><span class="event-year">%d</span>`, currentYear+1)
	if !strings.Contains(body, leftDate) || !strings.Contains(body, nextDate.Format("January 2, 2006")) {
		t.Fatalf("next-year event did not show the year in both date positions")
	}
	if !strings.Contains(body, "Tickets required") || !strings.Contains(body, "Not purchased") || !strings.Contains(body, "Tickets purchased") {
		t.Fatal("upcoming page did not show ticket status")
	}
}
