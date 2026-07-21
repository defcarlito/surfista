package surf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const DefaultSurflineBaseURL = "https://services.surfline.com"

var ErrEmptyQuery = errors.New("search query is empty")

type SurflineSearcher struct {
	baseURL *url.URL
	client  *http.Client
}

// NewSurflineSearcher configures Surfline's website search service. Surfline
// does not publish this as a supported public API, so the base URL is injectable
// and all endpoint/response assumptions remain isolated in this file.
func NewSurflineSearcher(baseURL string, client *http.Client) (*SurflineSearcher, error) {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = DefaultSurflineBaseURL
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse Surfline base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("Surfline base URL must include a scheme and host")
	}
	if client == nil {
		client = http.DefaultClient
	}

	return &SurflineSearcher{baseURL: parsed, client: client}, nil
}

func (s *SurflineSearcher) SearchSpots(ctx context.Context, query string) ([]Spot, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrEmptyQuery
	}

	endpoint := *s.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/search/site"
	params := endpoint.Query()
	params.Set("q", query)
	params.Set("querySize", "10")
	params.Set("suggestionSize", "10")
	params.Set("newsSearch", "true")
	params.Set("includeWavePools", "true")
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Surfline search request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search Surfline: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = http.StatusText(resp.StatusCode)
		}
		return nil, fmt.Errorf("Surfline search returned %d: %s", resp.StatusCode, detail)
	}

	var sections []surflineSection
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	if err := decoder.Decode(&sections); err != nil {
		return nil, fmt.Errorf("decode Surfline search response: %w", err)
	}

	spots := make([]Spot, 0)
	for _, section := range sections {
		for _, hit := range section.Hits.Hits {
			if hit.Type != "spot" || strings.TrimSpace(hit.ID) == "" || strings.TrimSpace(hit.Source.Name) == "" {
				continue
			}

			country := ""
			region := ""
			if len(hit.Source.Breadcrumbs) > 0 {
				country = hit.Source.Breadcrumbs[0]
			}
			if len(hit.Source.Breadcrumbs) > 1 {
				region = strings.Join(hit.Source.Breadcrumbs[1:], ", ")
			}

			spots = append(spots, Spot{
				ID:        hit.ID,
				Name:      hit.Source.Name,
				Region:    region,
				Country:   country,
				Latitude:  hit.Source.Location.Latitude,
				Longitude: hit.Source.Location.Longitude,
			})
		}
	}

	return spots, nil
}

type surflineSection struct {
	Hits struct {
		Hits []surflineHit `json:"hits"`
	} `json:"hits"`
}

type surflineHit struct {
	ID     string `json:"_id"`
	Type   string `json:"_type"`
	Source struct {
		Name        string   `json:"name"`
		Breadcrumbs []string `json:"breadCrumbs"`
		Location    struct {
			Latitude  float64 `json:"lat"`
			Longitude float64 `json:"lon"`
		} `json:"location"`
	} `json:"_source"`
}
