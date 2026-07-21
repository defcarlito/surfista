package search

import "surfista/internal/surf"

type SearchResultsMsg struct {
	RequestID uint64
	Query     string
	Spots     []surf.Spot
}

type SearchErrorMsg struct {
	RequestID uint64
	Query     string
	Err       error
}

type SpotAddedMsg struct {
	Spot  surf.Spot
	Added bool
	Err   error
}

type clearStatusMsg struct {
	StatusID uint64
}

type liveSearchMsg struct {
	RequestID uint64
	Query     string
}
