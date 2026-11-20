package event_test

import (
	"testing"
	"time"

	"events/internal/event"
)

// TestEventValidateAcceptsOptionalFields verifies that optional fields can be omitted.
func TestEventValidateAcceptsOptionalFields(t *testing.T) {
	item := event.Event{Name: "Community concert", Date: time.Date(2027, time.May, 16, 0, 0, 0, 0, time.UTC)}
	if err := item.Validate(); err != nil {
		t.Fatalf("Validate() returned an error: %v", err)
	}
}

// TestEventValidateRejectsInvalidLink verifies that an event link must use HTTP or HTTPS.
func TestEventValidateRejectsInvalidLink(t *testing.T) {
	item := event.Event{Name: "Community concert", Date: time.Date(2027, time.May, 16, 0, 0, 0, 0, time.UTC), Time: "19:00", Link: "javascript:alert(1)"}
	if err := item.Validate(); err == nil {
		t.Fatal("Validate() accepted a non-web link")
	}
}

// TestParseCostReadsWholeAmount verifies that whole dollar amounts convert to cents.
func TestParseCostReadsWholeAmount(t *testing.T) {
	cost, err := event.ParseCost("25")
	if err != nil || cost != 2500 {
		t.Fatalf("ParseCost() returned %d, error %v", cost, err)
	}
}

// TestParseCostReadsFractionalAmount verifies that decimal cost converts to cents.
func TestParseCostReadsFractionalAmount(t *testing.T) {
	cost, err := event.ParseCost("24.50")
	if err != nil || cost != 2450 {
		t.Fatalf("ParseCost() returned %d, error %v", cost, err)
	}
}

// TestParseCostReadsLeadingDecimal verifies that a fractional amount can omit its leading zero.
func TestParseCostReadsLeadingDecimal(t *testing.T) {
	cost, err := event.ParseCost(".5")
	if err != nil || cost != 50 {
		t.Fatalf("ParseCost() returned %d, error %v", cost, err)
	}
}

// TestParseCostRejectsExcessPrecision verifies that cost accepts no more than two decimal places.
func TestParseCostRejectsExcessPrecision(t *testing.T) {
	if _, err := event.ParseCost("12.345"); err == nil {
		t.Fatal("ParseCost() accepted three decimal places")
	}
}
