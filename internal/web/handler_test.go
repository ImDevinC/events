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
	handler, err := web.NewHandler(nil, metadata.NewFetcher(time.Second), time.UTC, 2*time.Second)
	if err != nil {
		t.Fatalf("NewHandler() returned an error: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/events/new", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	today := time.Now().UTC().Format("2006-01-02")
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `name="multiple_days"`) ||
		!strings.Contains(response.Body.String(), `name="date" type="date" value="`+today+`" required`) ||
		!strings.Contains(response.Body.String(), `name="end_date" type="date" value="`+today+`" required`) {
		t.Fatalf("new event form did not render both dates with the same default: status %d", response.Code)
	}

	assetRequest := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	assetResponse := httptest.NewRecorder()
	handler.ServeHTTP(assetResponse, assetRequest)
	if assetResponse.Code != http.StatusOK || !strings.Contains(assetResponse.Body.String(), `end_date`) || !strings.Contains(assetResponse.Body.String(), `event.preventDefault()`) {
		t.Fatalf("date-range validation script was not served: status %d", assetResponse.Code)
	}
}

// TestEventFormRejectsEndDateBeforeStart verifies invalid ranges are reported and cannot be submitted.
func TestEventFormRejectsEndDateBeforeStart(t *testing.T) {
	handler, err := web.NewHandler(nil, metadata.NewFetcher(time.Second), time.UTC, 2*time.Second)
	if err != nil {
		t.Fatalf("NewHandler() returned an error: %v", err)
	}
	form := url.Values{"name": {"Weekend festival"}, "date": {"2099-06-14"}, "end_date": {"2099-06-13"}}
	request := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "End date must be on or after the start date.") ||
		!strings.Contains(response.Body.String(), `class="button button-primary" type="submit" disabled`) {
		t.Fatalf("invalid end date was not blocked with a red form hint: status %d, body %q", response.Code, response.Body.String())
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
	multiName := "Multi-day test " + nameSuffix
	t.Cleanup(func() {
		_, err := cleanupDatabase.ExecContext(context.Background(), `DELETE FROM events WHERE name IN ($1, $2, $3)`, currentName, nextName, multiName)
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
		dateValue := date.Format("2006-01-02")
		form := url.Values{
			"name":     {name},
			"date":     {dateValue},
			"end_date": {dateValue},
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
	multiForm := url.Values{
		"name":     {multiName},
		"date":     {"2099-07-31"},
		"end_date": {"2099-08-01"},
	}
	multiRequest := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(multiForm.Encode()))
	multiRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	multiResponse := httptest.NewRecorder()
	handler.ServeHTTP(multiResponse, multiRequest)
	if multiResponse.Code != http.StatusSeeOther {
		t.Fatalf("POST multi-day /events returned status %d, body %q", multiResponse.Code, multiResponse.Body.String())
	}

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
	if !strings.Contains(body, "Saturday, July 31, 2099 – Sunday, August 1, 2099") {
		t.Fatal("upcoming page did not show the complete multi-day date range")
	}
	if !strings.Contains(body, `<span class="event-month">JUL</span><span class="event-day event-day-range">31 – 1</span>`) {
		t.Fatal("upcoming event's left date did not show the abbreviated date range")
	}

	calendarRequest := httptest.NewRequest(http.MethodGet, "/calendar?month=2099-07", nil)
	calendarResponse := httptest.NewRecorder()
	handler.ServeHTTP(calendarResponse, calendarRequest)
	if calendarResponse.Code != http.StatusOK {
		t.Fatalf("GET /calendar returned status %d", calendarResponse.Code)
	}
	if count := strings.Count(calendarResponse.Body.String(), `class="calendar-event" href="/events/`); count < 2 {
		t.Fatalf("calendar did not display the multi-day event on all of its dates (found %d event dates)", count)
	}
}
