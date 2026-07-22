package dashboard

import "surfista/internal/surf"

type ForecastLoadedMsg struct {
	SpotID   string
	Forecast surf.Forecast
	Err      error
}

type SpotRemovedMsg struct {
	SpotID  string
	Removed bool
	Err     error
}
