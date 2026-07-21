package dashboard

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"surfista/internal/surf"
)

const forecastTimeout = 20 * time.Second

type forecastState struct {
	forecast surf.Forecast
	loading  bool
	err      error
}

// Model owns the favorite-spots dashboard and its independently loading
// forecasts. Forecasts are keyed by Surfline's stable spot ID.
type Model struct {
	spots         []surf.Spot
	forecasts     map[string]forecastState
	provider      surf.ForecastProvider
	loadErr       error
	terminalWidth int
	selectedIndex int
	now           func() time.Time
}

func New(provider surf.ForecastProvider, spots []surf.Spot, loadErr error) Model {
	states := make(map[string]forecastState, len(spots))
	for _, spot := range spots {
		states[spot.ID] = forecastState{loading: provider != nil}
	}
	return Model{
		spots:         append([]surf.Spot(nil), spots...),
		forecasts:     states,
		provider:      provider,
		loadErr:       loadErr,
		selectedIndex: -1,
		now:           time.Now,
	}
}

func (m Model) Init() tea.Cmd {
	commands := make([]tea.Cmd, 0, len(m.spots))
	for _, spot := range m.spots {
		commands = append(commands, m.fetchForecast(spot.ID))
	}
	return tea.Batch(commands...)
}

// PendingForecasts reports how many favorite forecasts have not resolved yet.
func (m Model) PendingForecasts() int {
	pending := 0
	for _, state := range m.forecasts {
		if state.loading {
			pending++
		}
	}
	return pending
}

// Add makes a newly tracked spot visible immediately and starts its forecast.
func (m *Model) Add(spot surf.Spot) tea.Cmd {
	for _, tracked := range m.spots {
		if tracked.ID == spot.ID {
			return nil
		}
	}
	m.spots = append(m.spots, spot)
	m.forecasts[spot.ID] = forecastState{loading: m.provider != nil}
	return m.fetchForecast(spot.ID)
}

func (m Model) fetchForecast(spotID string) tea.Cmd {
	if m.provider == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), forecastTimeout)
		defer cancel()
		forecast, err := m.provider.Forecast(ctx, spotID)
		return ForecastLoadedMsg{SpotID: spotID, Forecast: forecast, Err: err}
	}
}
