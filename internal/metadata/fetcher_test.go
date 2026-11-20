package metadata_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"events/internal/metadata"
)

// TestParseReadsSchemaEventAndOpenGraph verifies that page metadata fills event fields.
func TestParseReadsSchemaEventAndOpenGraph(t *testing.T) {
	pageURL, err := url.Parse("https://example.com/shows/friday")
	if err != nil {
		t.Fatalf("Parse() test URL: %v", err)
	}
	page := `<html><head>
		<meta property="og:title" content="Live at the park">
		<meta property="og:image" content="/images/show.jpg">
		<script type="application/ld+json">{"@type":"Event","startDate":"2027-05-16T19:30:00-04:00","offers":{"price":"24.50"}}</script>
	</head></html>`
	data := metadata.Parse(page, pageURL)
	if data.Name != "Live at the park" || data.Date != "2027-05-16" || data.Time != "19:30" || data.Cost != "24.50" || data.ImageURL != "https://example.com/images/show.jpg" {
		t.Fatalf("Parse() returned unexpected event data: %#v", data)
	}
}

// TestParseRejectsPrivateThumbnail verifies that metadata cannot set a local network thumbnail.
func TestParseRejectsPrivateThumbnail(t *testing.T) {
	pageURL, err := url.Parse("https://example.com/events")
	if err != nil {
		t.Fatalf("Parse() test URL: %v", err)
	}
	data := metadata.Parse(`<meta property="og:image" content="http://127.0.0.1/private.png">`, pageURL)
	if data.ImageURL != "" {
		t.Fatalf("Parse() accepted a private thumbnail URL: %q", data.ImageURL)
	}
}

// TestFetcherRejectsPrivateAddress verifies that the fetcher blocks local network targets.
func TestFetcherRejectsPrivateAddress(t *testing.T) {
	fetcher := metadata.NewFetcher(time.Second)
	if _, err := fetcher.Fetch(context.Background(), "http://127.0.0.1:8000/private"); err == nil {
		t.Fatal("Fetch() accepted a private network URL")
	}
}

// TestFetcherRejectsSharedAddressSpace verifies that the fetcher blocks carrier-grade NAT addresses.
func TestFetcherRejectsSharedAddressSpace(t *testing.T) {
	fetcher := metadata.NewFetcher(time.Second)
	if _, err := fetcher.Fetch(context.Background(), "http://100.64.0.1/event"); err == nil {
		t.Fatal("Fetch() accepted a shared address space URL")
	}
}

// TestFetcherRejectsUniqueLocalIPv6 verifies that the fetcher blocks private IPv6 addresses.
func TestFetcherRejectsUniqueLocalIPv6(t *testing.T) {
	fetcher := metadata.NewFetcher(time.Second)
	if _, err := fetcher.Fetch(context.Background(), "http://[fd00::1]/event"); err == nil {
		t.Fatal("Fetch() accepted a unique local IPv6 URL")
	}
}

// TestFetcherRejectsUnsupportedScheme verifies that URL metadata rejects non-web protocols.
func TestFetcherRejectsUnsupportedScheme(t *testing.T) {
	fetcher := metadata.NewFetcher(time.Second)
	if _, err := fetcher.Fetch(context.Background(), "ftp://example.com/event"); err == nil {
		t.Fatal("Fetch() accepted a non-web URL scheme")
	}
}
