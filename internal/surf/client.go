package surf

import "context"

// SpotSearcher keeps the TUI independent from any particular search API.
type SpotSearcher interface {
	SearchSpots(ctx context.Context, query string) ([]Spot, error)
}
