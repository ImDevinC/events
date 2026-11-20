package web

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"events/internal/event"
	"events/internal/metadata"
	"events/internal/store"
)

//go:embed templates/*.html assets/*
var webFiles embed.FS

// Handler serves the event list, calendar, and event forms.
type Handler struct {
	store         *store.Postgres    // store reads and writes event records.
	fetcher       *metadata.Fetcher  // fetcher reads event page metadata.
	location      *time.Location     // location controls local event dates.
	healthTimeout time.Duration      // healthTimeout limits database health checks.
	templates     *template.Template // templates contains the embedded HTML templates.
}

// pageData contains the values needed to render a page.
type pageData struct {
	Title     string        // Title is the browser page title.
	ActiveTab string        // ActiveTab selects the active navigation item.
	Content   string        // Content selects the page template.
	Events    []eventCard   // Events contains the event cards shown on the page.
	Form      formData      // Form contains the event form values and messages.
	Month     calendarMonth // Month contains the calendar view data.
}

// eventCard contains display values for one event.
type eventCard struct {
	Event     event.Event // Event is the stored event.
	Day       string      // Day is the event day or abbreviated day range.
	Month     string      // Month is the short start month name.
	Year      string      // Year is set when the event is outside the current year.
	MultiDay  bool        // MultiDay indicates that the event spans more than one date.
	DateLabel string      // DateLabel is the readable event date.
	TimeLabel string      // TimeLabel is the readable event time.
	CostLabel string      // CostLabel is the formatted event cost.
	HasImage  bool        // HasImage indicates whether a thumbnail URL exists.
}

// formData contains event form values, messages, and edit state.
type formData struct {
	Event            event.Event // Event contains the form's event values.
	Date             string      // Date is the date input value.
	EndDate          string      // EndDate is the final date input value.
	DateRangeInvalid bool        // DateRangeInvalid indicates that the end date precedes the start date.
	Cost             string      // Cost is the decimal cost input value.
	Error            string      // Error is the validation or save message.
	Notice           string      // Notice is a non-blocking metadata message.
	ImageSourceLink  string      // ImageSourceLink records which link provided the image.
	IsEdit           bool        // IsEdit indicates whether the form edits a saved event.
}

// calendarMonth contains the month title, navigation links, and day cells.
type calendarMonth struct {
	Title    string        // Title is the readable month and year.
	Previous string        // Previous is the previous month in YYYY-MM format.
	Next     string        // Next is the next month in YYYY-MM format.
	Days     []calendarDay // Days contains the visible calendar cells.
}

// calendarDay contains the display values for one calendar day.
type calendarDay struct {
	Date      time.Time   // Date is the calendar date.
	DayNumber string      // DayNumber is the numeric day of the month.
	InMonth   bool        // InMonth indicates that the date belongs to the selected month.
	Today     bool        // Today indicates that the date is the current local date.
	Events    []eventCard // Events contains events scheduled on this date.
}

// NewHandler creates the HTTP routes and templates for the application.
func NewHandler(repository *store.Postgres, fetcher *metadata.Fetcher, location *time.Location, healthTimeout time.Duration) (http.Handler, error) {
	templates, err := template.ParseFS(webFiles, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse web templates: %w", err)
	}
	assets, err := fs.Sub(webFiles, "assets")
	if err != nil {
		return nil, fmt.Errorf("load web assets: %w", err)
	}
	handler := &Handler{store: repository, fetcher: fetcher, location: location, healthTimeout: healthTimeout, templates: templates}
	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	mux.HandleFunc("GET /healthz", handler.health)
	mux.HandleFunc("GET /{$}", handler.home)
	mux.HandleFunc("GET /calendar", handler.calendar)
	mux.HandleFunc("GET /events/new", handler.newEvent)
	mux.HandleFunc("POST /events/preview", handler.preview)
	mux.HandleFunc("POST /events", handler.create)
	mux.HandleFunc("GET /events/{id}/edit", handler.edit)
	mux.HandleFunc("POST /events/{id}", handler.update)
	return securityHeaders(limitRequestBody(mux)), nil
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.healthTimeout)
	defer cancel()
	if err := h.store.Ping(ctx); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	today := h.today()
	events, err := h.store.Upcoming(r.Context(), today)
	if err != nil {
		slog.Error("load upcoming events", "error", err)
		http.Error(w, "Could not load events.", http.StatusInternalServerError)
		return
	}
	data := pageData{Title: "Upcoming events", ActiveTab: "upcoming", Content: "home"}
	data.Events = h.cards(events)
	h.render(w, http.StatusOK, "layout", data)
}

func (h *Handler) newEvent(w http.ResponseWriter, r *http.Request) {
	today := h.today().Format("2006-01-02")
	data := pageData{Title: "Add an event", ActiveTab: "upcoming", Content: "new-event", Form: formData{Date: today, EndDate: today}}
	h.render(w, http.StatusOK, "layout", data)
}

func (h *Handler) preview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid event details.", http.StatusBadRequest)
		return
	}
	link := strings.TrimSpace(r.FormValue("link"))
	form := formData{Event: event.Event{Link: link}}
	if link == "" {
		form.Error = "Paste an event link to load its details."
	} else {
		metadata, err := h.fetcher.Fetch(r.Context(), link)
		if err != nil {
			form.Notice = "We could not read this page. Add the event details below."
			slog.Info("event metadata unavailable", "url", link, "error", err)
		} else {
			form.Event.Name = metadata.Name
			form.Event.Link = metadata.Link
			form.Event.ImageURL = metadata.ImageURL
			form.ImageSourceLink = metadata.Link
			form.Date = metadata.Date
			form.EndDate = metadata.Date
			form.Event.Time = metadata.Time
			form.Cost = metadata.Cost
		}
	}
	data := pageData{Title: "Review event details", ActiveTab: "upcoming", Content: "new-event", Form: form}
	if isHTMX(r) {
		h.render(w, http.StatusOK, "event-form", data)
		return
	}
	h.render(w, http.StatusOK, "layout", data)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	item, form, err := h.parseEvent(r)
	if err != nil {
		form.Error = err.Error()
		h.render(w, responseStatus(r, http.StatusUnprocessableEntity), responseTemplate(r, "event-form", "layout"), pageData{Title: "Add an event", ActiveTab: "upcoming", Content: "new-event", Form: form})
		return
	}
	h.addImageFromLink(r.Context(), &item)
	saved, err := h.store.Create(r.Context(), item)
	if err != nil {
		slog.Error("create event", "error", err)
		form.Error = "We could not save this event. Try again."
		h.render(w, responseStatus(r, http.StatusInternalServerError), responseTemplate(r, "event-form", "layout"), pageData{Title: "Add an event", ActiveTab: "upcoming", Content: "new-event", Form: form})
		return
	}
	h.redirectAfterSave(w, r, saved.ID)
}

func (h *Handler) edit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	item, err := h.store.Get(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("load event for edit", "event_id", id, "error", err)
		http.Error(w, "Could not load this event.", http.StatusInternalServerError)
		return
	}
	data := pageData{Title: "Edit event", ActiveTab: "upcoming", Content: "edit-event", Form: h.formFor(item, true)}
	h.render(w, http.StatusOK, "layout", data)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r.PathValue("id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	item, form, err := h.parseEvent(r)
	item.ID = id
	form.Event.ID = id
	form.IsEdit = true
	if err != nil {
		form.Error = err.Error()
		h.render(w, responseStatus(r, http.StatusUnprocessableEntity), responseTemplate(r, "event-form", "layout"), pageData{Title: "Edit event", ActiveTab: "upcoming", Content: "edit-event", Form: form})
		return
	}
	h.addImageFromLink(r.Context(), &item)
	saved, err := h.store.Update(r.Context(), item)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		slog.Error("update event", "event_id", id, "error", err)
		form.Error = "We could not save this event. Try again."
		h.render(w, responseStatus(r, http.StatusInternalServerError), responseTemplate(r, "event-form", "layout"), pageData{Title: "Edit event", ActiveTab: "upcoming", Content: "edit-event", Form: form})
		return
	}
	h.redirectAfterSave(w, r, saved.ID)
}

func (h *Handler) calendar(w http.ResponseWriter, r *http.Request) {
	month, err := h.requestedMonth(r.URL.Query().Get("month"))
	if err != nil {
		http.Error(w, "Month must use YYYY-MM format.", http.StatusBadRequest)
		return
	}
	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, h.location)
	gridStart := start.AddDate(0, 0, -int(start.Weekday()))
	gridEnd := gridStart.AddDate(0, 0, 42)
	events, err := h.store.Between(r.Context(), gridStart, gridEnd)
	if err != nil {
		slog.Error("load calendar events", "error", err)
		http.Error(w, "Could not load the calendar.", http.StatusInternalServerError)
		return
	}
	data := pageData{Title: "Event calendar", ActiveTab: "calendar", Content: "calendar", Month: h.makeCalendar(month, events)}
	if isHTMX(r) {
		h.render(w, http.StatusOK, "calendar-grid", data)
		return
	}
	h.render(w, http.StatusOK, "layout", data)
}

func (h *Handler) parseEvent(r *http.Request) (event.Event, formData, error) {
	if err := r.ParseForm(); err != nil {
		return event.Event{}, formData{}, errors.New("event form is not valid")
	}
	dateValue := strings.TrimSpace(r.FormValue("date"))
	endDateValue := strings.TrimSpace(r.FormValue("end_date"))
	form := formData{
		Date:    dateValue,
		EndDate: endDateValue,
		Cost:    strings.TrimSpace(r.FormValue("cost")),
		Event: event.Event{
			Name:             strings.TrimSpace(r.FormValue("name")),
			Time:             strings.TrimSpace(r.FormValue("time")),
			Link:             strings.TrimSpace(r.FormValue("link")),
			ImageURL:         strings.TrimSpace(r.FormValue("image_url")),
			TicketsRequired:  r.FormValue("tickets_required") == "true",
			TicketsPurchased: r.FormValue("tickets_purchased") == "true",
		},
		ImageSourceLink: strings.TrimSpace(r.FormValue("image_source_link")),
	}
	if form.ImageSourceLink != form.Event.Link {
		form.Event.ImageURL = ""
	}
	if dateValue != "" {
		date, err := time.ParseInLocation("2006-01-02", dateValue, h.location)
		if err != nil {
			return event.Event{}, form, errors.New("event date must be a valid date")
		}
		form.Event.Date = date
	}
	if endDateValue == "" {
		return event.Event{}, form, errors.New("event end date is required")
	}
	endDate, err := time.ParseInLocation("2006-01-02", endDateValue, h.location)
	if err != nil {
		return event.Event{}, form, errors.New("event end date must be a valid date")
	}
	if !form.Event.Date.IsZero() && endDate.Before(form.Event.Date) {
		form.DateRangeInvalid = true
		return event.Event{}, form, errors.New("end date must be on or after the start date")
	}
	if !endDate.Equal(form.Event.Date) {
		form.Event.EndDate = endDate
	}
	if form.Cost != "" {
		cost, err := event.ParseCost(form.Cost)
		if err != nil {
			return event.Event{}, form, err
		}
		form.Event.CostCents = &cost
	}
	if err := form.Event.Validate(); err != nil {
		return event.Event{}, form, err
	}
	return form.Event, form, nil
}

func (h *Handler) addImageFromLink(ctx context.Context, item *event.Event) {
	if item.Link == "" || item.ImageURL != "" {
		return
	}
	metadata, err := h.fetcher.Fetch(ctx, item.Link)
	if err != nil {
		slog.Info("event image unavailable", "url", item.Link, "error", err)
		return
	}
	item.ImageURL = metadata.ImageURL
}

func (h *Handler) formFor(item event.Event, isEdit bool) formData {
	form := formData{Event: item, Date: item.Date.Format("2006-01-02"), EndDate: item.Date.Format("2006-01-02"), IsEdit: isEdit}
	if !item.EndDate.IsZero() {
		form.EndDate = item.EndDate.Format("2006-01-02")
	}
	if item.ImageURL != "" {
		form.ImageSourceLink = item.Link
	}
	if item.CostCents != nil {
		form.Cost = fmt.Sprintf("%d.%02d", *item.CostCents/100, *item.CostCents%100)
	}
	return form
}

func (h *Handler) cards(items []event.Event) []eventCard {
	cards := make([]eventCard, 0, len(items))
	for _, item := range items {
		cards = append(cards, h.card(item))
	}
	return cards
}

func (h *Handler) card(item event.Event) eventCard {
	yearDiffers := item.Date.Year() != h.today().Year()
	month := strings.ToUpper(item.Date.Format("Jan"))
	dateLabel := item.Date.Format("Monday, January 2")
	multiDay := !item.EndDate.IsZero()
	year := ""
	if yearDiffers {
		year = fmt.Sprintf("%d", item.Date.Year())
		dateLabel += fmt.Sprintf(", %d", item.Date.Year())
	}
	if multiDay {
		startYear := item.Date.Year() != h.today().Year() || item.Date.Year() != item.EndDate.Year()
		endYear := item.EndDate.Year() != h.today().Year() || item.Date.Year() != item.EndDate.Year()
		dateLabel = item.Date.Format("Monday, January 2")
		if startYear {
			dateLabel += fmt.Sprintf(", %d", item.Date.Year())
		}
		dateLabel += " – " + item.EndDate.Format("Monday, January 2")
		if endYear {
			dateLabel += fmt.Sprintf(", %d", item.EndDate.Year())
		}
	}
	card := eventCard{
		Event:     item,
		Day:       item.Date.Format("02"),
		Month:     month,
		Year:      year,
		MultiDay:  multiDay,
		DateLabel: dateLabel,
		HasImage:  item.ImageURL != "",
	}
	if multiDay {
		card.Month = strings.ToUpper(item.Date.Format("Jan"))
		card.Day = item.Date.Format("2") + " – " + item.EndDate.Format("2")
		if item.Date.Year() != item.EndDate.Year() {
			if yearDiffers || item.EndDate.Year() != h.today().Year() {
				card.Year = fmt.Sprintf("%d – %d", item.Date.Year(), item.EndDate.Year())
			}
		} else if yearDiffers {
			card.Year = fmt.Sprintf("%d", item.Date.Year())
		}
	}
	if item.Time != "" {
		parsed, err := time.Parse("15:04", item.Time)
		if err == nil {
			card.TimeLabel = parsed.Format("3:04 PM")
		}
	} else {
		card.TimeLabel = "Time to be announced"
	}
	if item.CostCents != nil {
		card.CostLabel = fmt.Sprintf("$%d.%02d", *item.CostCents/100, *item.CostCents%100)
	}
	return card
}

func (h *Handler) makeCalendar(month time.Time, events []event.Event) calendarMonth {
	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, h.location)
	gridStart := start.AddDate(0, 0, -int(start.Weekday()))
	today := h.today().Format("2006-01-02")
	days := make([]calendarDay, 42)
	byDate := make(map[string][]eventCard, len(events))
	for index := range days {
		date := gridStart.AddDate(0, 0, index)
		key := date.Format("2006-01-02")
		for _, item := range events {
			lastDate := item.EndDate
			if lastDate.IsZero() {
				lastDate = item.Date
			}
			if key >= item.Date.Format("2006-01-02") && key <= lastDate.Format("2006-01-02") {
				byDate[key] = append(byDate[key], h.card(item))
			}
		}
		days[index] = calendarDay{
			Date:      date,
			DayNumber: strconv.Itoa(date.Day()),
			InMonth:   date.Month() == month.Month(),
			Today:     key == today,
			Events:    byDate[key],
		}
	}
	return calendarMonth{
		Title:    start.Format("January 2006"),
		Previous: start.AddDate(0, -1, 0).Format("2006-01"),
		Next:     start.AddDate(0, 1, 0).Format("2006-01"),
		Days:     days,
	}
}

func (h *Handler) requestedMonth(raw string) (time.Time, error) {
	if raw == "" {
		today := h.today()
		return time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, h.location), nil
	}
	month, err := time.ParseInLocation("2006-01", raw, h.location)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, h.location), nil
}

func (h *Handler) today() time.Time {
	now := time.Now().In(h.location)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, h.location)
}

func (h *Handler) render(w http.ResponseWriter, status int, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("render page", "template", name, "error", err)
	}
}

func (h *Handler) redirectAfterSave(w http.ResponseWriter, r *http.Request, id int64) {
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/events/%d/edit", id), http.StatusSeeOther)
}

func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid event ID")
	}
	return id, nil
}

func responseTemplate(r *http.Request, htmx string, page string) string {
	if isHTMX(r) {
		return htmx
	}
	return page
}

func responseStatus(r *http.Request, status int) int {
	if isHTMX(r) {
		return http.StatusOK
	}
	return status
}

func isHTMX(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' https: http: data:; script-src 'self'; style-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func limitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		}
		next.ServeHTTP(w, r)
	})
}
