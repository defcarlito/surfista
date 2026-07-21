package surf

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

const testSpotSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>https://www.surfline.com/surf-report/honolua-bay/5842041f4e65fad6a7708de4</loc></url>
	<url><loc>https://www.surfline.com/surf-report/honolua-bay/5842041f4e65fad6a7708de4/forecast</loc></url>
	<url><loc>https://www.surfline.com/surf-report/honoli-i/5842041f4e65fad6a7708dec</loc></url>
	<url><loc>https://example.com/surf-report/not-surfline/5842041f4e65fad6a7708def</loc></url>
</urlset>`

func TestSitemapSearchMapsDeduplicatesAndCachesSpots(t *testing.T) {
	t.Parallel()

	requests := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.Path != "/sitemaps/spots.xml" {
			t.Fatalf("request path = %q, want /sitemaps/spots.xml", request.URL.Path)
		}
		return xmlResponse(http.StatusOK, testSpotSitemap), nil
	})}

	searcher, err := NewSitemapSearcher("https://www.surfline.com/sitemaps/spots.xml", httpClient)
	if err != nil {
		t.Fatal(err)
	}

	spots, err := searcher.SearchSpots(context.Background(), " honolua ")
	if err != nil {
		t.Fatal(err)
	}
	if len(spots) != 1 {
		t.Fatalf("got %d spots, want 1: %+v", len(spots), spots)
	}
	want := Spot{
		ID:   "5842041f4e65fad6a7708de4",
		Name: "Honolua Bay",
		URL:  "https://www.surfline.com/surf-report/honolua-bay/5842041f4e65fad6a7708de4",
	}
	if spots[0] != want {
		t.Fatalf("spot = %+v, want %+v", spots[0], want)
	}

	if _, err := searcher.SearchSpots(context.Background(), "Honoli'i"); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("sitemap requests = %d, want 1", requests)
	}
}

func TestSitemapSearchRejectsEmptyQueryWithoutRequest(t *testing.T) {
	t.Parallel()

	requested := false
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requested = true
		return xmlResponse(http.StatusOK, testSpotSitemap), nil
	})}
	searcher, err := NewSitemapSearcher("https://www.surfline.com/sitemaps/spots.xml", httpClient)
	if err != nil {
		t.Fatal(err)
	}

	_, err = searcher.SearchSpots(context.Background(), " \t\n ")
	if !errors.Is(err, ErrEmptyQuery) {
		t.Fatalf("error = %v, want ErrEmptyQuery", err)
	}
	if requested {
		t.Fatal("empty query downloaded the sitemap")
	}
}

func TestSitemapSearchErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "HTTP error", statusCode: http.StatusBadGateway, body: `upstream unavailable`},
		{name: "malformed response", statusCode: http.StatusOK, body: `<urlset><url>`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return xmlResponse(test.statusCode, test.body), nil
			})}
			searcher, err := NewSitemapSearcher("https://www.surfline.com/sitemaps/spots.xml", httpClient)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := searcher.SearchSpots(context.Background(), "Pipeline"); err == nil {
				t.Fatal("SearchSpots() error = nil, want an error")
			}
		})
	}
}

func TestSitemapSearchRanksAndLimitsMatches(t *testing.T) {
	t.Parallel()

	catalog := []Spot{
		{ID: "contains", Name: "Little Honolua Bay"},
		{ID: "exact", Name: "Honolua Bay"},
		{ID: "prefix", Name: "Honolua Bay North"},
		{ID: "terms", Name: "Bay near Honolua"},
	}
	spots := searchCatalog(catalog, "Honolua Bay")
	wantOrder := []string{"exact", "prefix", "contains", "terms"}
	for index, wantID := range wantOrder {
		if spots[index].ID != wantID {
			t.Fatalf("result %d ID = %q, want %q", index, spots[index].ID, wantID)
		}
	}
}

func TestParseSurflineReportURLRejectsLookalikes(t *testing.T) {
	t.Parallel()

	tests := []string{
		"https://example.com/surf-report/honolua-bay/5842041f4e65fad6a7708de4",
		"https://surfline.example.com/surf-report/honolua-bay/5842041f4e65fad6a7708de4",
		"https://www.surfline.com/surf-report/honolua-bay/not-a-spot-id",
		"javascript:alert(1)",
	}
	for _, candidate := range tests {
		if _, _, _, ok := parseSurflineReportURL(candidate); ok {
			t.Errorf("accepted invalid Surfline URL %q", candidate)
		}
	}
}

func xmlResponse(status int, body string) *http.Response {
	response := jsonResponse(status, body)
	response.Header.Set("Content-Type", "application/xml; charset=UTF-8")
	return response
}

func TestNormalizeSearchText(t *testing.T) {
	t.Parallel()
	if got := normalizeSearchText(" Honoli'i--Bay "); got != "honoli i bay" {
		t.Fatalf("normalizeSearchText() = %q", got)
	}
	if !strings.Contains(normalizeSearchText("Huntington State Beach"), "huntington") {
		t.Fatal("normalized name lost searchable text")
	}
}
