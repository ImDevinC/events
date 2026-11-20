package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxPageSize = 2 << 20

var (
	attributePattern = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	jsonLDPattern    = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script\s*>`)
	metaPattern      = regexp.MustCompile(`(?is)<meta\b([^>]*)>`)
	titlePattern     = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title\s*>`)
	tagPattern       = regexp.MustCompile(`(?is)<[^>]+>`)
	spacePattern     = regexp.MustCompile(`\s+`)
)

var restrictedIPv4Networks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
}

var globalIPv6Network = netip.MustParsePrefix("2000::/3")

var restrictedIPv6Networks = []netip.Prefix{
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

// EventData contains event details found on a web page.
type EventData struct {
	Name     string // Name is the event name found on the page.
	Date     string // Date is the event date in YYYY-MM-DD format.
	Time     string // Time is the optional event time in HH:MM format.
	Cost     string // Cost is the optional event price as text.
	ImageURL string // ImageURL is the optional event thumbnail URL.
	Link     string // Link is the final page URL after redirects.
}

// Fetcher safely downloads public web pages to read event metadata.
type Fetcher struct {
	client *http.Client // client performs bounded requests to public web pages.
}

// NewFetcher creates a metadata fetcher with a request timeout.
func NewFetcher(timeout time.Duration) *Fetcher {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("resolve host: %w", err)
			}
			if len(addresses) == 0 {
				return nil, errors.New("host has no IP addresses")
			}
			var lastError error
			for _, address := range addresses {
				if !isPublicIP(address.IP) {
					return nil, errors.New("host resolves to a non-public IP address")
				}
				connection, err := (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
				if err == nil {
					return connection, nil
				}
				lastError = err
			}
			return nil, fmt.Errorf("connect to host: %w", lastError)
		},
	}
	return &Fetcher{client: &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return validateTarget(request.URL)
		},
	}}
}

// Fetch reads event fields and an image URL from a public web page.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) (EventData, error) {
	if len(rawURL) > 2048 {
		return EventData{}, errors.New("event URL must be 2048 characters or fewer")
	}
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return EventData{}, errors.New("enter a valid event URL")
	}
	if err := validateTarget(parsedURL); err != nil {
		return EventData{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return EventData{}, fmt.Errorf("create metadata request: %w", err)
	}
	request.Header.Set("User-Agent", "events/1.0 (+event metadata preview)")
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	response, err := f.client.Do(request)
	if err != nil {
		return EventData{}, fmt.Errorf("fetch event page: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return EventData{}, fmt.Errorf("event page returned HTTP %d", response.StatusCode)
	}
	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		return EventData{}, errors.New("event URL did not return an HTML page")
	}
	page, err := io.ReadAll(io.LimitReader(response.Body, maxPageSize+1))
	if err != nil {
		return EventData{}, fmt.Errorf("read event page: %w", err)
	}
	if len(page) > maxPageSize {
		return EventData{}, errors.New("event page is larger than 2 MB")
	}
	data := Parse(string(page), response.Request.URL)
	data.Link = response.Request.URL.String()
	return data, nil
}

func isPublicIP(ip net.IP) bool {
	address, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsUnspecified() {
		return false
	}
	if address.Is4() {
		for _, network := range restrictedIPv4Networks {
			if network.Contains(address) {
				return false
			}
		}
		return true
	}
	if !globalIPv6Network.Contains(address) {
		return false
	}
	for _, network := range restrictedIPv6Networks {
		if network.Contains(address) {
			return false
		}
	}
	return true
}

func validateTarget(target *url.URL) error {
	if target.Scheme != "http" && target.Scheme != "https" {
		return errors.New("event URL must use HTTP or HTTPS")
	}
	if target.User != nil {
		return errors.New("event URL cannot contain credentials")
	}
	host := target.Hostname()
	if host == "" {
		return errors.New("event URL must include a host")
	}
	if ip := net.ParseIP(host); ip != nil && !isPublicIP(ip) {
		return errors.New("event URL cannot target a non-public IP address")
	}
	return nil
}

// Parse extracts common event metadata from an HTML page.
func Parse(page string, pageURL *url.URL) EventData {
	data := EventData{Link: pageURL.String()}
	metadata := make(map[string]string)
	for _, match := range metaPattern.FindAllStringSubmatch(page, -1) {
		attributes := parseAttributes(match[1])
		key := strings.ToLower(firstNonEmpty(attributes["property"], attributes["name"], attributes["itemprop"]))
		if key != "" {
			metadata[key] = cleanText(attributes["content"])
		}
	}
	data.Name = firstNonEmpty(metadata["og:title"], metadata["twitter:title"], metadata["name"], metadata["title"])
	if data.Name == "" {
		if match := titlePattern.FindStringSubmatch(page); len(match) > 1 {
			data.Name = cleanText(match[1])
		}
	}
	data.ImageURL = resolveURL(pageURL, firstNonEmpty(metadata["og:image"], metadata["og:image:url"], metadata["twitter:image"], metadata["image"]))
	if start := firstNonEmpty(metadata["event:start_time"], metadata["startdate"]); start != "" {
		setDateTime(&data, start)
	}
	if price := firstNonEmpty(metadata["product:price:amount"], metadata["event:price"], metadata["price"]); price != "" {
		data.Cost = price
	}
	for _, match := range jsonLDPattern.FindAllStringSubmatch(page, -1) {
		if !strings.EqualFold(parseAttributes(match[1])["type"], "application/ld+json") {
			continue
		}
		var value any
		if json.Unmarshal([]byte(match[2]), &value) == nil {
			if applyEventData(&data, value, pageURL) {
				break
			}
		}
	}
	return data
}

func parseAttributes(raw string) map[string]string {
	attributes := make(map[string]string)
	for _, match := range attributePattern.FindAllStringSubmatch(raw, -1) {
		value := firstNonEmpty(match[2], match[3], match[4])
		attributes[strings.ToLower(match[1])] = value
	}
	return attributes
}

func applyEventData(data *EventData, value any, base *url.URL) bool {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if applyEventData(data, item, base) {
				return true
			}
		}
	case map[string]any:
		if graph, ok := typed["@graph"]; ok && applyEventData(data, graph, base) {
			return true
		}
		typeName := strings.ToLower(fmt.Sprint(typed["@type"]))
		if strings.Contains(typeName, "event") {
			if name, ok := typed["name"].(string); ok && data.Name == "" {
				data.Name = cleanText(name)
			}
			if start, ok := typed["startDate"].(string); ok && data.Date == "" {
				setDateTime(data, start)
			}
			if data.ImageURL == "" {
				data.ImageURL = resolveURL(base, resolveURLFromValue(typed["image"]))
			}
			if data.Cost == "" {
				data.Cost = offerPrice(typed["offers"])
			}
			return true
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if applyEventData(data, typed[key], base) {
				return true
			}
		}
	}
	return false
}

func cleanText(value string) string {
	value = tagPattern.ReplaceAllString(value, " ")
	value = html.UnescapeString(value)
	return strings.TrimSpace(spacePattern.ReplaceAllString(value, " "))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func offerPrice(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		return firstNonEmpty(asString(typed["price"]), asString(typed["lowPrice"]))
	case []any:
		for _, offer := range typed {
			if price := offerPrice(offer); price != "" {
				return price
			}
		}
	}
	return ""
}

func asString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

func resolveURL(base *url.URL, raw string) string {
	if raw == "" || base == nil {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "" && parsed.Scheme != "https" && parsed.Scheme != "http") {
		return ""
	}
	resolved := base.ResolveReference(parsed)
	if validateTarget(resolved) != nil {
		return ""
	}
	return resolved.String()
}

func resolveURLFromValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]any:
		return asString(typed["url"])
	case []any:
		for _, item := range typed {
			if image := resolveURLFromValue(item); image != "" {
				return image
			}
		}
	}
	return ""
}

func setDateTime(data *EventData, raw string) {
	var parsed time.Time
	var err error
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		parsed, err = time.Parse(layout, raw)
		if err == nil {
			break
		}
	}
	if err == nil {
		data.Date = parsed.Format("2006-01-02")
		if strings.Contains(raw, "T") && len(raw) >= 16 {
			data.Time = parsed.Format("15:04")
		}
	}
}
