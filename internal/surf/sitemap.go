package surf

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"unicode"
)

const (
	DefaultSpotSitemapURL = "https://www.surfline.com/sitemaps/spots.xml"
	maxSitemapBytes       = 16 << 20
	maxSearchResults      = 10
)

// SitemapSearcher searches Surfline's public spot sitemap. The sitemap is
// downloaded at most once per process and then searched locally, so typing and
// repeated searches do not depend on a rate-limited third-party search page.
type SitemapSearcher struct {
	sitemapURL *url.URL
	client     *http.Client

	mu      sync.Mutex
	loaded  bool
	catalog []Spot
}

func NewSitemapSearcher(sitemapURL string, client *http.Client) (*SitemapSearcher, error) {
	if strings.TrimSpace(sitemapURL) == "" {
		sitemapURL = DefaultSpotSitemapURL
	}

	parsed, err := url.Parse(sitemapURL)
	if err != nil {
		return nil, fmt.Errorf("parse Surfline spot sitemap URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("Surfline spot sitemap URL must include a scheme and host")
	}
	if client == nil {
		client = http.DefaultClient
	}

	return &SitemapSearcher{sitemapURL: parsed, client: client}, nil
}

func (s *SitemapSearcher) SearchSpots(ctx context.Context, query string) ([]Spot, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrEmptyQuery
	}

	catalog, err := s.spots(ctx)
	if err != nil {
		return nil, err
	}
	return searchCatalog(catalog, query), nil
}

func (s *SitemapSearcher) spots(ctx context.Context) ([]Spot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loaded {
		return s.catalog, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.sitemapURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Surfline spot catalog request: %w", err)
	}
	req.Header.Set("Accept", "application/xml,text/xml")
	req.Header.Set("User-Agent", "Surfista/0.1 (+local spot search)")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download Surfline spot catalog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Surfline spot catalog returned %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	catalog, err := parseSpotSitemap(io.LimitReader(resp.Body, maxSitemapBytes))
	if err != nil {
		return nil, fmt.Errorf("decode Surfline spot catalog: %w", err)
	}

	s.catalog = catalog
	s.loaded = true
	return s.catalog, nil
}

type sitemapURLSet struct {
	URLs []struct {
		Location string `xml:"loc"`
	} `xml:"url"`
}

func parseSpotSitemap(reader io.Reader) ([]Spot, error) {
	var document sitemapURLSet
	if err := xml.NewDecoder(reader).Decode(&document); err != nil {
		return nil, err
	}

	spots := make([]Spot, 0, len(document.URLs)/2)
	seen := make(map[string]struct{})
	for _, entry := range document.URLs {
		id, slug, canonicalURL, ok := parseSurflineReportURL(strings.TrimSpace(entry.Location))
		if !ok {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		spots = append(spots, Spot{
			ID:   id,
			Name: spotNameFromSlug(slug),
			URL:  canonicalURL,
		})
	}
	return spots, nil
}

type rankedSpot struct {
	spot  Spot
	score int
}

func searchCatalog(catalog []Spot, query string) []Spot {
	normalizedQuery := normalizeSearchText(query)
	if normalizedQuery == "" {
		return nil
	}
	queryTerms := strings.Fields(normalizedQuery)
	matches := make([]rankedSpot, 0)

	for _, spot := range catalog {
		name := normalizeSearchText(spot.Name)
		score, ok := catalogMatchScore(name, normalizedQuery, queryTerms)
		if ok {
			matches = append(matches, rankedSpot{spot: spot, score: score})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score < matches[j].score
		}
		if matches[i].spot.Name != matches[j].spot.Name {
			return matches[i].spot.Name < matches[j].spot.Name
		}
		return matches[i].spot.ID < matches[j].spot.ID
	})

	if len(matches) > maxSearchResults {
		matches = matches[:maxSearchResults]
	}
	spots := make([]Spot, len(matches))
	for index, match := range matches {
		spots[index] = match.spot
	}
	return spots
}

func catalogMatchScore(name, query string, queryTerms []string) (int, bool) {
	switch {
	case name == query:
		return 0, true
	case strings.HasPrefix(name, query):
		return 1, true
	case strings.Contains(name, query):
		return 2, true
	}

	for _, term := range queryTerms {
		if !strings.Contains(name, term) {
			return 0, false
		}
	}
	return 3, true
}

func normalizeSearchText(value string) string {
	return strings.Join(strings.FieldsFunc(strings.ToLower(value), func(character rune) bool {
		return !unicode.IsLetter(character) && !unicode.IsNumber(character)
	}), " ")
}

func parseSurflineReportURL(raw string) (id, slug, canonical string, ok bool) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "", "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "surfline.com" && host != "www.surfline.com" {
		return "", "", "", false
	}

	parts := strings.Split(strings.Trim(parsed.EscapedPath(), "/"), "/")
	if len(parts) < 3 || parts[0] != "surf-report" || !spotIDPattern(parts[2]) {
		return "", "", "", false
	}
	slug, err = url.PathUnescape(parts[1])
	if err != nil || slug == "" || strings.Contains(slug, "/") {
		return "", "", "", false
	}
	id = strings.ToLower(parts[2])
	canonical = "https://www.surfline.com/surf-report/" + url.PathEscape(slug) + "/" + id
	return id, slug, canonical, true
}

func spotIDPattern(value string) bool {
	if len(value) != 24 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9') &&
			!(character >= 'a' && character <= 'f') &&
			!(character >= 'A' && character <= 'F') {
			return false
		}
	}
	return true
}

func spotNameFromSlug(slug string) string {
	words := strings.Fields(strings.ReplaceAll(strings.Trim(slug, "-"), "-", " "))
	for index, word := range words {
		runes := []rune(word)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
			words[index] = string(runes)
		}
	}
	return strings.Join(words, " ")
}
