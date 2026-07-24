package app

import (
	"surfista/internal/screens/dashboard"
	"surfista/internal/screens/loading"
	"surfista/internal/screens/search"
	"surfista/internal/surf"
)

type screen int

const (
	loadingScreen screen = iota
	homeScreen
	searchScreen
)

type Model struct {
	current          screen
	search           search.Model
	dashboard        dashboard.Model
	loading          loading.Model
	initialForecasts int
}

type Tracker interface {
	search.Tracker
	dashboard.Remover
}

func New(searcher surf.SpotSearcher, tracker Tracker, forecaster surf.ForecastProvider, tracked []surf.Spot, loadErr error) Model {
	dashboardModel := dashboard.New(forecaster, tracker, tracked, loadErr)
	initialForecasts := dashboardModel.PendingInitialFetches()
	current := homeScreen
	if initialForecasts > 0 {
		current = loadingScreen
	}
	return Model{
		current:          current,
		search:           search.New(searcher, tracker),
		dashboard:        dashboardModel,
		loading:          loading.New(initialForecasts),
		initialForecasts: initialForecasts,
	}
}
