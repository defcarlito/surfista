package surf

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestSurflineResponseMapping(t *testing.T) {
	t.Parallel()

	var request *http.Request
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		request = r
		return jsonResponse(http.StatusOK, `[
			{"hits":{"hits":[
				{"_id":"5842041f4e65fad6a7708827","_type":"spot","_source":{
					"name":"Huntington State Beach",
					"breadCrumbs":["United States","California","Orange County"],
					"location":{"lat":33.654,"lon":-118.003}
				}},
				{"_id":"not-a-spot","_type":"subregion","_source":{"name":"Orange County"}}
			]}}
		]`), nil
	})}

	client, err := NewSurflineSearcher("https://example.test", httpClient)
	if err != nil {
		t.Fatal(err)
	}
	spots, err := client.SearchSpots(context.Background(), " Huntington Beach ")
	if err != nil {
		t.Fatal(err)
	}
	if request.URL.Path != "/search/site" {
		t.Fatalf("path = %q, want /search/site", request.URL.Path)
	}
	if query := request.URL.Query().Get("q"); query != "Huntington Beach" {
		t.Fatalf("q = %q, want Huntington Beach", query)
	}
	wantParams := map[string]string{
		"querySize":        "10",
		"suggestionSize":   "10",
		"newsSearch":       "true",
		"includeWavePools": "true",
	}
	for name, want := range wantParams {
		if got := request.URL.Query().Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	if len(spots) != 1 {
		t.Fatalf("got %d spots, want 1", len(spots))
	}
	got := spots[0]
	if got.ID != "5842041f4e65fad6a7708827" || got.Name != "Huntington State Beach" {
		t.Fatalf("unexpected spot identity: %+v", got)
	}
	if got.Country != "United States" || got.Region != "California, Orange County" {
		t.Fatalf("unexpected location mapping: %+v", got)
	}
	if got.Latitude != 33.654 || got.Longitude != -118.003 {
		t.Fatalf("unexpected coordinates: %+v", got)
	}
}

func TestSurflineSearchRejectsEmptyQuery(t *testing.T) {
	t.Parallel()

	client, err := NewSurflineSearcher("https://example.com", http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.SearchSpots(context.Background(), " \t\n ")
	if !errors.Is(err, ErrEmptyQuery) {
		t.Fatalf("error = %v, want ErrEmptyQuery", err)
	}
}

func TestSurflineSearchErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "API error", statusCode: http.StatusBadGateway, body: `upstream unavailable`},
		{name: "malformed response", statusCode: http.StatusOK, body: `{not-json`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return jsonResponse(test.statusCode, test.body), nil
			})}

			client, err := NewSurflineSearcher("https://example.test", httpClient)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.SearchSpots(context.Background(), "Pipeline"); err == nil {
				t.Fatal("SearchSpots() error = nil, want an error")
			}
		})
	}
}
