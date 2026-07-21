package app

import (
	"surfista/internal/screens/dashboard"
	"surfista/internal/screens/search"
	"surfista/internal/surf"
)

type screen int

const (
	homeScreen screen = iota
	searchScreen
)

type Model struct {
	current   screen
	search    search.Model
	dashboard dashboard.Model
}

func New(searcher surf.SpotSearcher, tracker search.Tracker, forecaster surf.ForecastProvider, tracked []surf.Spot, loadErr error) Model {
	return Model{
		current:   homeScreen,
		search:    search.New(searcher, tracker),
		dashboard: dashboard.New(forecaster, tracked, loadErr),
	}
}
