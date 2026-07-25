package dashboard

import (
	"context"
	"errors"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"surfista/internal/surf"
)

const (
	forecastTimeout        = 20 * time.Second
	maxConcurrentForecasts = 2
)

type forecastState struct {
	forecast        surf.Forecast
	updatedAt       time.Time
	fetchDisplayAt  time.Time
	fetchDisplaySet bool
	queued          bool
	loading         bool
	fetched         bool
	manualRefresh   bool
	refreshFailed   bool
	err             error
}

type forecastDetailsState struct {
	details   surf.ForecastDetails
	updatedAt time.Time
	queued    bool
	loading   bool
	fetched   bool
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
	refreshSpinner    spinner.Model
	forecastQueue     []string
	detailsQueue      []string
	viewMode          dashboardViewMode
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
	initialForecasts := make([]string, 0, len(spots))
	for _, spot := range spots {
		entry, hasCache := cached[spot.ID]
		hasCache = hasCache && (!entry.ForecastUpdatedAt.IsZero() || len(entry.Forecast.Slots) > 0)
		states[spot.ID] = forecastState{
			forecast:  entry.Forecast,
			updatedAt: entry.ForecastUpdatedAt,
		}
		detailStates[spot.ID] = forecastDetailsState{
			details:   entry.Details,
			updatedAt: entry.DetailsUpdatedAt,
		}
		if !hasCache && provider != nil {
			initialForecasts = append(initialForecasts, spot.ID)
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
		refreshSpinner: spinner.New(
			spinner.WithSpinner(spinner.MiniDot),
		),
	}
	for index, spotID := range initialForecasts {
		state := model.forecasts[spotID]
		if index < maxConcurrentForecasts {
			state.loading = true
		} else {
			state.queued = true
			model.forecastQueue = append(model.forecastQueue, spotID)
		}
		model.forecasts[spotID] = state
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

func (m Model) HasUsableForecasts() bool {
	for _, state := range m.forecasts {
		if state.usable() {
			return true
		}
	}
	return false
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
	queued := m.forecastQueue[:0]
	for _, queuedSpotID := range m.forecastQueue {
		if queuedSpotID != spotID {
			queued = append(queued, queuedSpotID)
		}
	}
	m.forecastQueue = queued
	queuedDetails := m.detailsQueue[:0]
	for _, queuedSpotID := range m.detailsQueue {
		if queuedSpotID != spotID {
			queuedDetails = append(queuedDetails, queuedSpotID)
		}
	}
	m.detailsQueue = queuedDetails
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
		if m.forecasts[spot.ID].loading {
			commands = append(commands, m.fetchForecast(spot.ID))
		}
		if m.details[spot.ID].loading {
			commands = append(commands, m.fetchForecastDetails(spot.ID))
		}
	}
	return tea.Batch(commands...)
}

func (m Model) canRefresh() bool {
	return len(m.spots) > 0 && m.provider != nil && !m.hasPendingForecasts()
}

func (m *Model) refresh() tea.Cmd {
	if !m.canRefresh() {
		return nil
	}
	m.forecastQueue = m.forecastQueue[:0]
	for _, spot := range m.spots {
		state := m.forecasts[spot.ID]
		state.fetchDisplayAt = state.updatedAt
		state.fetchDisplaySet = true
		state.queued = true
		state.loading = false
		state.err = nil
		state.refreshFailed = false
		state.manualRefresh = true
		m.forecasts[spot.ID] = state
		m.forecastQueue = append(m.forecastQueue, spot.ID)
	}
	commands := m.dequeueQueuedForecasts()
	commands = append(commands, m.refreshSpinner.Tick)
	return tea.Batch(commands...)
}

func (m Model) spotFetching(spotID string) bool {
	return m.forecasts[spotID].loading
}

func (m Model) hasActiveFetches() bool {
	for _, spot := range m.spots {
		if m.spotFetching(spot.ID) {
			return true
		}
	}
	return false
}

func (m Model) hasActiveAnimations() bool {
	if m.hasActiveFetches() {
		return true
	}
	for _, state := range m.details {
		if state.loading {
			return true
		}
	}
	return false
}

func (m *Model) startQueuedDetails() tea.Cmd {
	return tea.Batch(m.dequeueQueuedDetails()...)
}

func (m *Model) dequeueQueuedDetails() []tea.Cmd {
	available := maxConcurrentForecasts
	for _, state := range m.details {
		if state.loading {
			available--
		}
	}

	commands := make([]tea.Cmd, 0, max(0, available))
	for available > 0 && len(m.detailsQueue) > 0 {
		spotID := m.detailsQueue[0]
		m.detailsQueue = m.detailsQueue[1:]
		state, tracked := m.details[spotID]
		if !tracked || !state.queued {
			continue
		}
		state.queued = false
		state.loading = true
		m.details[spotID] = state
		commands = append(commands, m.fetchForecastDetails(spotID))
		available--
	}
	return commands
}

func (m *Model) queueMissingWindDetails() tea.Cmd {
	if m.detailsProvider == nil {
		return nil
	}

	startSpinner := !m.hasActiveAnimations()
	for _, spot := range m.spots {
		state := m.details[spot.ID]
		if len(state.details.Slots) > 0 || state.loading || state.queued {
			continue
		}
		state.queued = true
		state.err = nil
		m.details[spot.ID] = state
		m.detailsQueue = append(m.detailsQueue, spot.ID)
	}

	commands := m.dequeueQueuedDetails()
	if startSpinner && len(commands) > 0 {
		commands = append(commands, m.refreshSpinner.Tick)
	}
	return tea.Batch(commands...)
}

func (m *Model) queueSelectedDetails(spotID string) tea.Cmd {
	state := m.details[spotID]
	if state.loading {
		return nil
	}
	if m.detailsProvider == nil {
		state.err = errors.New("detailed forecast is unavailable")
		m.details[spotID] = state
		return nil
	}

	startSpinner := !m.hasActiveAnimations()
	if state.queued {
		kept := m.detailsQueue[:0]
		for _, queuedSpotID := range m.detailsQueue {
			if queuedSpotID != spotID {
				kept = append(kept, queuedSpotID)
			}
		}
		m.detailsQueue = kept
	}
	state.queued = true
	state.err = nil
	m.details[spotID] = state
	m.detailsQueue = append([]string{spotID}, m.detailsQueue...)

	commands := m.dequeueQueuedDetails()
	if startSpinner && len(commands) > 0 {
		commands = append(commands, m.refreshSpinner.Tick)
	}
	return tea.Batch(commands...)
}

func (m Model) hasPendingForecasts() bool {
	for _, state := range m.forecasts {
		if state.loading || state.queued {
			return true
		}
	}
	return false
}

func (m *Model) startQueuedForecasts() tea.Cmd {
	return tea.Batch(m.dequeueQueuedForecasts()...)
}

func (m *Model) dequeueQueuedForecasts() []tea.Cmd {
	available := maxConcurrentForecasts
	for _, state := range m.forecasts {
		if state.loading {
			available--
		}
	}

	commands := make([]tea.Cmd, 0, max(0, available))
	for available > 0 && len(m.forecastQueue) > 0 {
		spotID := m.forecastQueue[0]
		m.forecastQueue = m.forecastQueue[1:]
		state := m.forecasts[spotID]
		state.queued = false
		state.loading = true
		m.forecasts[spotID] = state
		commands = append(commands, m.fetchForecast(spotID))
		available--
	}
	return commands
}

// FetchSpinnerTick starts the dashboard's per-spot fetch animation when
// startup leaves the loading screen before every request has resolved.
func (m Model) FetchSpinnerTick() tea.Cmd {
	if !m.hasActiveFetches() {
		return nil
	}
	return m.refreshSpinner.Tick
}

func (m *Model) finishSpotFetch(spotID string) {
	if m.spotFetching(spotID) {
		return
	}
	state, tracked := m.forecasts[spotID]
	if !tracked {
		return
	}
	state.fetchDisplayAt = time.Time{}
	state.fetchDisplaySet = false
	state.manualRefresh = false
	m.forecasts[spotID] = state
}

// PendingInitialFetches reports how many startup Surfline requests are still
// in flight. Cached values remain usable fallback data, but do not count as a
// completed refresh.
func (m Model) PendingInitialFetches() int {
	pending := 0
	for _, state := range m.forecasts {
		if state.loading || state.queued {
			pending++
		}
	}
	return pending
}

// InitialFetchProgress reports how many startup summaries have resolved.
func (m Model) InitialFetchProgress() int {
	loaded := 0
	for _, state := range m.forecasts {
		if !state.loading && !state.queued {
			loaded++
		}
	}
	return loaded
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
	m.forecasts[spot.ID] = forecastState{queued: m.provider != nil}
	m.details[spot.ID] = forecastDetailsState{}
	if m.provider != nil {
		m.forecastQueue = append(m.forecastQueue, spot.ID)
	}
	m.applySort()
	forecastCmd := m.startQueuedForecasts()
	if m.viewMode == dashboardViewWind {
		return tea.Batch(forecastCmd, m.queueMissingWindDetails())
	}
	return forecastCmd
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
	return m.queueSelectedDetails(spot.ID)
}
