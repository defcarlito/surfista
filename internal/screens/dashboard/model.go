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
	forecast  surf.Forecast
	updatedAt time.Time
	loading   bool
	err       error
}

type forecastDetailsState struct {
	details   surf.ForecastDetails
	updatedAt time.Time
	loading   bool
	err       error
}

func (s forecastState) usable() bool {
	return !s.updatedAt.IsZero() || len(s.forecast.Slots) > 0
}

func (s forecastDetailsState) usable() bool {
	return !s.updatedAt.IsZero() || s.details.SpotID != "" || len(s.details.Slots) > 0 || len(s.details.Tides) > 0 || len(s.details.Sunlight) > 0
}

type Remover interface {
	Remove(spotID string) (bool, error)
}

type ForecastCache interface {
	LoadForecastCache() (map[string]surf.ForecastCacheEntry, error)
	SaveForecastCache(entry surf.ForecastCacheEntry) error
}

// Model owns the favorite-spots dashboard and its independently loading
// forecasts. Forecasts are keyed by Surfline's stable spot ID.
type Model struct {
	spots             []surf.Spot
	forecasts         map[string]forecastState
	provider          surf.ForecastProvider
	detailsProvider   surf.ForecastDetailsProvider
	details           map[string]forecastDetailsState
	forecastCache     ForecastCache
	remover           Remover
	loadErr           error
	terminalWidth     int
	terminalHeight    int
	forecastDayOffset int
	selectedIndex     int
	scrollOffset      int
	confirmRemoval    bool
	removing          bool
	removalSpot       surf.Spot
	removalErr        error
	detailsOpen       bool
	detailsSpot       surf.Spot
	detailsScroll     int
	openURL           func(string) error
	sortMode          SortMode
	sortStore         SortStore
	addedOrder        map[string]int
	nextAddedOrder    int
	now               func() time.Time
}

func New(provider surf.ForecastProvider, remover Remover, spots []surf.Spot, loadErr error) Model {
	detailsProvider, _ := provider.(surf.ForecastDetailsProvider)
	forecastCache, _ := remover.(ForecastCache)
	cached := map[string]surf.ForecastCacheEntry{}
	if forecastCache != nil {
		if loaded, err := forecastCache.LoadForecastCache(); err == nil {
			cached = loaded
		}
	}
	states := make(map[string]forecastState, len(spots))
	detailStates := make(map[string]forecastDetailsState, len(spots))
	for _, spot := range spots {
		entry := cached[spot.ID]
		states[spot.ID] = forecastState{
			forecast:  entry.Forecast,
			updatedAt: entry.ForecastUpdatedAt,
			loading:   provider != nil,
		}
		detailStates[spot.ID] = forecastDetailsState{
			details:   entry.Details,
			updatedAt: entry.DetailsUpdatedAt,
			loading:   detailsProvider != nil,
		}
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
		spots:           append([]surf.Spot(nil), spots...),
		forecasts:       states,
		provider:        provider,
		detailsProvider: detailsProvider,
		details:         detailStates,
		forecastCache:   forecastCache,
		remover:         remover,
		loadErr:         loadErr,
		selectedIndex:   -1,
		openURL:         systemOpenURL,
		sortMode:        sortMode,
		sortStore:       sortStore,
		addedOrder:      addedOrder,
		nextAddedOrder:  len(spots),
		now:             time.Now,
	}
	model.applySort()
	return model
}

func (m Model) ConfirmingRemoval() bool {
	return m.confirmRemoval
}

func (m Model) ShowingDetails() bool {
	return m.detailsOpen
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
	delete(m.details, spotID)
	delete(m.addedOrder, spotID)
	m.clampForecastDayOffset()
	m.clampScrollOffset()
	m.selectedIndex = -1
	m.confirmRemoval = false
	m.removing = false
	m.removalSpot = surf.Spot{}
	m.removalErr = nil
	if m.detailsSpot.ID == spotID {
		m.detailsOpen = false
		m.detailsSpot = surf.Spot{}
		m.detailsScroll = 0
	}
}

func (m Model) Init() tea.Cmd {
	commands := make([]tea.Cmd, 0, len(m.spots)*2)
	for _, spot := range m.spots {
		commands = append(commands, m.fetchForecast(spot.ID))
		commands = append(commands, m.fetchForecastDetails(spot.ID))
	}
	return tea.Batch(commands...)
}

// PendingForecasts reports how many favorite forecasts have not resolved yet.
func (m Model) PendingForecasts() int {
	pending := 0
	for _, state := range m.forecasts {
		if state.loading && state.updatedAt.IsZero() {
			pending++
		}
	}
	for _, state := range m.details {
		if state.loading && state.updatedAt.IsZero() {
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
	m.details[spot.ID] = forecastDetailsState{loading: m.detailsProvider != nil}
	m.applySort()
	return tea.Batch(m.fetchForecast(spot.ID), m.fetchForecastDetails(spot.ID))
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

func (m Model) fetchForecastDetails(spotID string) tea.Cmd {
	if m.detailsProvider == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), forecastTimeout)
		defer cancel()
		details, err := m.detailsProvider.ForecastDetails(ctx, spotID)
		return ForecastDetailsLoadedMsg{SpotID: spotID, Details: details, Err: err}
	}
}

func (m Model) saveForecastCache(spotID string) {
	if m.forecastCache == nil {
		return
	}
	forecast := m.forecasts[spotID]
	details := m.details[spotID]
	_ = m.forecastCache.SaveForecastCache(surf.ForecastCacheEntry{
		SpotID:            spotID,
		Forecast:          forecast.forecast,
		ForecastUpdatedAt: forecast.updatedAt,
		Details:           details.details,
		DetailsUpdatedAt:  details.updatedAt,
	})
}

func (m *Model) openSelectedDetails() tea.Cmd {
	if !m.HasSelection() {
		return nil
	}

	spot := m.spots[m.selectedIndex]
	m.detailsOpen = true
	m.detailsSpot = spot
	m.resetDetailsScroll()
	state, cached := m.details[spot.ID]
	if cached && (state.loading || (state.err == nil && state.usable())) {
		return nil
	}
	if m.detailsProvider == nil {
		m.details[spot.ID] = forecastDetailsState{err: errors.New("detailed forecast is unavailable")}
		return nil
	}
	state.loading = true
	state.err = nil
	m.details[spot.ID] = state
	return m.fetchForecastDetails(spot.ID)
}
