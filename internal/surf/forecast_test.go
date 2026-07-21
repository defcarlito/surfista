package surf

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSurflineForecastMergesWaveAndRatingSlots(t *testing.T) {
	t.Parallel()

	requests := make(chan *http.Request, 2)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests <- r.Clone(context.Background())
		var body string
		switch r.URL.Path {
		case "/kbyg/spots/forecasts/wave":
			body = `{"associated":{"utcOffset":-10},"data":{"wave":[
				{"timestamp":200,"surf":{"min":2,"max":3,"plus":true,"humanRelation":"2-3 ft +"}},
				{"timestamp":100,"surf":{"min":1,"max":2,"plus":false,"humanRelation":"1-2 ft"}},
				{"timestamp":300,"surf":{"min":4,"max":5}}
			]}}`
		case "/kbyg/spots/forecasts/rating":
			body = `{"associated":{"utcOffset":-10},"data":{"rating":[
				{"timestamp":100,"rating":{"key":"POOR","value":0}},
				{"timestamp":200,"rating":{"key":"FAIR_TO_GOOD","value":3}}
			]}}`
		default:
			return forecastHTTPResponse(http.StatusNotFound, "text/plain", "not found"), nil
		}
		return forecastHTTPResponse(http.StatusOK, "application/json; charset=utf-8", body), nil
	})}

	provider, err := NewSurflineForecastProvider("https://example.test", httpClient)
	if err != nil {
		t.Fatal(err)
	}
	forecast, err := provider.Forecast(context.Background(), " honolua-id ")
	if err != nil {
		t.Fatal(err)
	}

	if forecast.SpotID != "honolua-id" {
		t.Fatalf("spot ID = %q, want honolua-id", forecast.SpotID)
	}
	if forecast.UTCOffset != -10*time.Hour {
		t.Fatalf("UTC offset = %v, want -10h", forecast.UTCOffset)
	}
	if len(forecast.Slots) != 2 {
		t.Fatalf("slots = %d, want 2 matching slots", len(forecast.Slots))
	}
	if got := forecast.Slots[0]; got.Timestamp.Unix() != 100 || got.Rating != "Poor" || got.SurfHeight.HumanRelation != "1-2 ft" {
		t.Fatalf("first slot = %+v", got)
	}
	if got := forecast.Slots[1]; got.Timestamp.Unix() != 200 || got.Rating != "Fair to Good" || !got.SurfHeight.Plus {
		t.Fatalf("second slot = %+v", got)
	}

	seen := map[string]*http.Request{}
	for range 2 {
		request := <-requests
		seen[request.URL.Path] = request
	}
	for path, request := range seen {
		query := request.URL.Query()
		if query.Get("spotId") != "honolua-id" || query.Get("days") != "2" || query.Get("intervalHours") != "3" {
			t.Errorf("%s query = %v", path, query)
		}
		if request.Header.Get("Accept") != "application/json" {
			t.Errorf("%s Accept = %q", path, request.Header.Get("Accept"))
		}
		if request.Header.Get("User-Agent") != surfistaUserAgent {
			t.Errorf("%s User-Agent = %q", path, request.Header.Get("User-Agent"))
		}
	}
	if seen["/kbyg/spots/forecasts/wave"].URL.Query().Get("maxHeights") != "false" {
		t.Error("wave request did not request full height data")
	}
	if seen["/kbyg/spots/forecasts/rating"].URL.Query().Get("cacheEnabled") != "true" {
		t.Error("rating request did not enable the forecast cache")
	}
}

func TestSurflineForecastUsesRawHeightFallback(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/wave") {
			return forecastHTTPResponse(http.StatusOK, "application/json", `{"data":{"wave":[{"timestamp":100,"surf":{"raw":{"min":1.25,"max":2.75}}}]}}`), nil
		}
		return forecastHTTPResponse(http.StatusOK, "application/json", `{"data":{"rating":[{"timestamp":100,"rating":{"value":2}}]}}`), nil
	})}

	provider, err := NewSurflineForecastProvider("https://example.test", httpClient)
	if err != nil {
		t.Fatal(err)
	}
	forecast, err := provider.Forecast(context.Background(), "spot-id")
	if err != nil {
		t.Fatal(err)
	}
	got := forecast.Slots[0]
	if got.SurfHeight.Min != 1.25 || got.SurfHeight.Max != 2.75 || got.Rating != "Fair" {
		t.Fatalf("slot = %+v", got)
	}
}

func TestSurflineForecastRejectsEmptySpotID(t *testing.T) {
	t.Parallel()

	provider, err := NewSurflineForecastProvider("https://example.test", http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Forecast(context.Background(), " \t\n ")
	if !errors.Is(err, ErrEmptySpotID) {
		t.Fatalf("error = %v, want ErrEmptySpotID", err)
	}
}

func TestSurflineForecastReportsNonJSONResponse(t *testing.T) {
	t.Parallel()

	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return forecastHTTPResponse(http.StatusOK, "text/html; charset=utf-8", `<html>challenge</html>`), nil
	})}

	provider, err := NewSurflineForecastProvider("https://example.test", httpClient)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Forecast(context.Background(), "spot-id")
	if err == nil || !strings.Contains(err.Error(), "unexpected Content-Type") {
		t.Fatalf("error = %v, want unexpected Content-Type error", err)
	}
}

func forecastHTTPResponse(status int, contentType, body string) *http.Response {
	header := make(http.Header)
	header.Set("Content-Type", contentType)
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
