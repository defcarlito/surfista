package surf

import "context"

// SpotSearcher keeps the TUI independent from any particular search API.
type SpotSearcher interface {
	SearchSpots(ctx context.Context, query string) ([]Spot, error)
}

// ForecastProvider fetches the small slice of forecast data needed by the
// dashboard without exposing Surfline's response format to the TUI.
type ForecastProvider interface {
	Forecast(ctx context.Context, spotID string) (Forecast, error)
}
