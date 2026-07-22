package search

import (
	"net/url"
	"strings"

	"surfista/internal/surf"
)

func deduplicateSpotsByURL(spots []surf.Spot) []surf.Spot {
	unique := make([]surf.Spot, 0, len(spots))
	seenURLs := make(map[string]struct{}, len(spots))

	for _, spot := range spots {
		key := normalizedSpotURL(spot.URL)
		if key != "" {
			if _, duplicate := seenURLs[key]; duplicate {
				continue
			}
			seenURLs[key] = struct{}{}
		}
		unique = append(unique, spot)
	}

	return unique
}

func normalizedSpotURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return strings.TrimRight(rawURL, "/")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String()
}
