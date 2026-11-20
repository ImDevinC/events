package web

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParseEventEndDateDefinesSingleOrMultiDayEvent(t *testing.T) {
	handler := &Handler{location: time.UTC}
	for _, test := range []struct {
		name           string
		endDate        string
		wantEndDate    string
		wantRangeError bool
	}{
		{name: "single day", endDate: "2099-06-12"},
		{name: "multi day", endDate: "2099-06-14", wantEndDate: "2099-06-14"},
		{name: "end before start", endDate: "2099-06-11", wantRangeError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			form := url.Values{
				"name":     {"Weekend festival"},
				"date":     {"2099-06-12"},
				"end_date": {test.endDate},
			}
			request := httptest.NewRequest("POST", "/events", strings.NewReader(form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			item, data, err := handler.parseEvent(request)
			if test.wantRangeError {
				if err == nil || !data.DateRangeInvalid {
					t.Fatalf("parseEvent() error = %v, DateRangeInvalid = %t; want invalid range", err, data.DateRangeInvalid)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseEvent() returned an error: %v", err)
			}
			if test.wantEndDate == "" {
				if !item.EndDate.IsZero() {
					t.Errorf("single-day EndDate = %s, want zero", item.EndDate)
				}
			} else if got := item.EndDate.Format("2006-01-02"); got != test.wantEndDate {
				t.Errorf("EndDate = %q, want %q", got, test.wantEndDate)
			}
		})
	}
}
