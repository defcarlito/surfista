package dashboard

import (
	"context"
	"errors"
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

type Remover interface {
	Remove(spotID string) (bool, error)
}

// Model owns the favorite-spots dashboard and its independently loading
// forecasts. Forecasts are keyed by Surfline's stable spot ID.
type Model struct {
	spots          []surf.Spot
	forecasts      map[string]forecastState
	provider       surf.ForecastProvider
	remover        Remover
	loadErr        error
	terminalWidth  int
	terminalHeight int
	selectedIndex  int
	scrollOffset   int
	confirmRemoval bool
	removing       bool
	removalSpot    surf.Spot
	removalErr     error
	openURL        func(string) error
	sortMode       SortMode
	sortStore      SortStore
	addedOrder     map[string]int
	nextAddedOrder int
	now            func() time.Time
}

func New(provider surf.ForecastProvider, remover Remover, spots []surf.Spot, loadErr error) Model {
	states := make(map[string]forecastState, len(spots))
	for _, spot := range spots {
		states[spot.ID] = forecastState{loading: provider != nil}
	}
	addedOrder := make(map[string]int, len(spots))
	for index, spot := range spots {
		addedOrder[spot.ID] = index
	}

	sortMode := SortTimeAdded
	var sortStore SortStore
	if candidate, ok := remover.(SortStore); ok {
		sortStore = candidate
		if persisted, err := candidate.LoadSortMode(); err == nil {
			if loadedMode, valid := parseSortMode(persisted); valid {
				sortMode = loadedMode
			}
		}
	}

	model := Model{
		spots:          append([]surf.Spot(nil), spots...),
		forecasts:      states,
		provider:       provider,
		remover:        remover,
		loadErr:        loadErr,
		selectedIndex:  -1,
		openURL:        systemOpenURL,
		sortMode:       sortMode,
		sortStore:      sortStore,
		addedOrder:     addedOrder,
		nextAddedOrder: len(spots),
		now:            time.Now,
	}
	model.applySort()
	return model
}

func (m Model) ConfirmingRemoval() bool {
	return m.confirmRemoval
}

func (m Model) HasSelection() bool {
	return m.selectedIndex >= 0 && m.selectedIndex < len(m.spots)
}

func (m Model) removeCmd(spotID string) tea.Cmd {
	return func() tea.Msg {
		if m.remover == nil {
			return SpotRemovedMsg{SpotID: spotID, Err: errors.New("tracked location storage is unavailable")}
		}
		removed, err := m.remover.Remove(spotID)
		return SpotRemovedMsg{SpotID: spotID, Removed: removed, Err: err}
	}
}

func (m *Model) removeSpot(spotID string) {
	kept := make([]surf.Spot, 0, max(0, len(m.spots)-1))
	for _, spot := range m.spots {
		if spot.ID != spotID {
			kept = append(kept, spot)
		}
	}
	m.spots = kept
	delete(m.forecasts, spotID)
	delete(m.addedOrder, spotID)
	m.clampScrollOffset()
	m.selectedIndex = -1
	m.confirmRemoval = false
	m.removing = false
	m.removalSpot = surf.Spot{}
	m.removalErr = nil
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
	m.addedOrder[spot.ID] = m.nextAddedOrder
	m.nextAddedOrder++
	m.forecasts[spot.ID] = forecastState{loading: m.provider != nil}
	m.applySort()
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
