package event

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Event is a saved upcoming event.
type Event struct {
	ID               int64     // ID is the unique event identifier.
	Name             string    // Name is the event name.
	Date             time.Time // Date is the local calendar date of the event.
	Time             string    // Time is the optional event time in HH:MM format.
	TicketsRequired  bool      // TicketsRequired indicates that the event requires a ticket.
	TicketsPurchased bool      // TicketsPurchased indicates that a ticket was purchased.
	CostCents        *int64    // CostCents is the optional cost in cents.
	Link             string    // Link is the optional source page URL.
	ImageURL         string    // ImageURL is the optional event thumbnail URL.
}

// Validate checks event fields before they are stored.
func (e Event) Validate() error {
	if strings.TrimSpace(e.Name) == "" {
		return errors.New("event name is required")
	}
	if len(e.Name) > 200 {
		return errors.New("event name must be 200 characters or fewer")
	}
	if e.Date.IsZero() {
		return errors.New("event date is required")
	}
	if e.Time != "" {
		if _, err := time.Parse("15:04", e.Time); err != nil {
			return errors.New("event time must use HH:MM format")
		}
	}
	if e.CostCents != nil && *e.CostCents < 0 {
		return errors.New("cost cannot be negative")
	}
	if e.CostCents != nil && *e.CostCents > 9_999_999_999 {
		return errors.New("cost is too high")
	}
	if err := validateWebURL(e.Link); err != nil {
		return fmt.Errorf("event link: %w", err)
	}
	if err := validateWebURL(e.ImageURL); err != nil {
		return fmt.Errorf("event image URL: %w", err)
	}
	return nil
}

// ParseCost converts a non-negative decimal amount with up to two digits after the decimal point to cents.
func ParseCost(value string) (int64, error) {
	if strings.HasPrefix(value, "-") {
		return 0, errors.New("cost cannot be negative")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return 0, errors.New("cost must be a number with up to two decimal places")
	}
	wholeText := parts[0]
	if wholeText == "" && len(parts) == 2 {
		wholeText = "0"
	}
	if wholeText == "" {
		return 0, errors.New("cost must be a number with up to two decimal places")
	}
	for _, digit := range wholeText {
		if digit < '0' || digit > '9' {
			return 0, errors.New("cost must be a number with up to two decimal places")
		}
	}
	whole, err := strconv.ParseInt(wholeText, 10, 64)
	if err != nil || whole > 99999999 {
		return 0, errors.New("cost must be a number with up to two decimal places")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if fraction == "" || len(fraction) > 2 {
			return 0, errors.New("cost must have no more than two decimal places")
		}
		for _, digit := range fraction {
			if digit < '0' || digit > '9' {
				return 0, errors.New("cost must be a number with up to two decimal places")
			}
		}
	}
	if len(fraction) == 1 {
		fraction += "0"
	}
	cents := int64(0)
	if fraction != "" {
		cents, _ = strconv.ParseInt(fraction, 10, 64)
	}
	return whole*100 + cents, nil
}

func validateWebURL(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	if len(rawURL) > 2048 {
		return errors.New("must be 2048 characters or fewer")
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" {
		return errors.New("must be an HTTP or HTTPS URL")
	}
	if parsed.User != nil {
		return errors.New("cannot contain credentials")
	}
	return nil
}
