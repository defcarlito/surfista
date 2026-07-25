package dashboard

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"surfista/internal/surf"
	"surfista/internal/ui"
)

type fakeForecastProvider struct {
	forecast      surf.Forecast
	err           error
	spotIDs       []string
	details       surf.ForecastDetails
	detailsErr    error
	detailSpotIDs []string
}

type fakeForecastCache struct {
	entries map[string]surf.ForecastCacheEntry
	saved   []surf.ForecastCacheEntry
}

func (c *fakeForecastCache) Remove(string) (bool, error) {
	return true, nil
}

func (c *fakeForecastCache) LoadForecastCache() (map[string]surf.ForecastCacheEntry, error) {
	return c.entries, nil
}

func (c *fakeForecastCache) SaveForecastCache(entry surf.ForecastCacheEntry) error {
	c.saved = append(c.saved, entry)
	return nil
}

func (p *fakeForecastProvider) Forecast(_ context.Context, spotID string) (surf.Forecast, error) {
	p.spotIDs = append(p.spotIDs, spotID)
	return p.forecast, p.err
}

func (p *fakeForecastProvider) ForecastDetails(_ context.Context, spotID string) (surf.ForecastDetails, error) {
	p.detailSpotIDs = append(p.detailSpotIDs, spotID)
	return p.details, p.detailsErr
}

func TestInitFetchesFavoriteForecast(t *testing.T) {
	t.Parallel()

	provider := &fakeForecastProvider{forecast: surf.Forecast{SpotID: "honolua"}}
	model := New(provider, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	cmd := model.Init()
	if cmd == nil {
		t.Fatal("Init returned no forecast command")
	}

	messages := commandMessages(t, cmd)
	var loaded ForecastLoadedMsg
	found := false
	for _, message := range messages {
		if candidate, ok := message.(ForecastLoadedMsg); ok {
			loaded = candidate
			found = true
		}
	}
	if !found {
		t.Fatalf("messages = %T, want ForecastLoadedMsg", messages)
	}
	if loaded.SpotID != "honolua" || loaded.Forecast.SpotID != "honolua" || loaded.Err != nil {
		t.Fatalf("message = %+v", loaded)
	}
	if len(provider.spotIDs) != 1 || provider.spotIDs[0] != "honolua" {
		t.Fatalf("provider calls = %v", provider.spotIDs)
	}
	if len(provider.detailSpotIDs) != 0 {
		t.Fatalf("startup unexpectedly prefetched details for %v", provider.detailSpotIDs)
	}
}

func TestInitLimitsConcurrentUncachedForecasts(t *testing.T) {
	t.Parallel()

	spots := []surf.Spot{
		{ID: "first"}, {ID: "second"}, {ID: "third"}, {ID: "fourth"}, {ID: "fifth"},
	}
	model := New(&fakeForecastProvider{}, nil, spots, nil)
	if active, queued := forecastWorkCounts(model); active != 2 || queued != 3 {
		t.Fatalf("initial work = %d active, %d queued; want 2 active, 3 queued", active, queued)
	}
	messages := commandMessages(t, model.Init())
	forecasts := 0
	for _, message := range messages {
		switch message.(type) {
		case ForecastLoadedMsg:
			forecasts++
		case ForecastDetailsLoadedMsg:
			t.Fatal("uncached startup unexpectedly fetched details")
		}
	}
	if forecasts != 2 {
		t.Fatalf("initial forecast commands = %d, want 2", forecasts)
	}
}

func TestForecastResultUpdatesOnlyTrackedSpot(t *testing.T) {
	t.Parallel()

	model := New(nil, nil, []surf.Spot{{ID: "honolua"}}, nil)
	failure := errors.New("forecast offline")
	updated, _ := model.Update(ForecastLoadedMsg{SpotID: "honolua", Err: failure})
	if !errors.Is(updated.forecasts["honolua"].err, failure) {
		t.Fatalf("forecast error = %v", updated.forecasts["honolua"].err)
	}

	updated, _ = updated.Update(ForecastLoadedMsg{SpotID: "not-tracked", Forecast: surf.Forecast{SpotID: "not-tracked"}})
	if _, exists := updated.forecasts["not-tracked"]; exists {
		t.Fatal("accepted a forecast for an untracked spot")
	}
}

func TestCachedForecastsDoNotStartInitialRefreshes(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	cache := &fakeForecastCache{entries: map[string]surf.ForecastCacheEntry{
		"honolua": {
			SpotID:            "honolua",
			Forecast:          surf.Forecast{SpotID: "honolua", Slots: []surf.ForecastSlot{{Rating: "Fair"}}},
			ForecastUpdatedAt: updatedAt,
			Details:           surf.ForecastDetails{SpotID: "honolua", Slots: []surf.ForecastDetailSlot{{Timestamp: updatedAt}}},
			DetailsUpdatedAt:  updatedAt,
		},
	}}
	provider := &fakeForecastProvider{}
	model := New(provider, cache, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)

	if pending := model.PendingInitialFetches(); pending != 0 {
		t.Fatalf("pending initial fetches = %d, want 0 with a complete cache", pending)
	}
	if got := model.forecasts["honolua"]; got.updatedAt != updatedAt || got.forecast.Slots[0].Rating != "Fair" || got.loading {
		t.Fatalf("hydrated forecast state = %+v", got)
	}
	if got := model.details["honolua"]; got.updatedAt != updatedAt || len(got.details.Slots) != 1 || got.loading {
		t.Fatalf("hydrated detail state = %+v", got)
	}
	if model.Init() != nil {
		t.Fatal("complete cache unexpectedly produced startup refresh commands")
	}
}

func TestInitialFetchProgressCountsDashboardForecasts(t *testing.T) {
	t.Parallel()

	spots := []surf.Spot{{ID: "first"}, {ID: "second"}}
	model := New(&fakeForecastProvider{}, nil, spots, nil)

	if loaded := model.InitialFetchProgress(); loaded != 0 {
		t.Fatalf("initial fetch progress = %d, want 0", loaded)
	}

	model, _ = model.Update(ForecastLoadedMsg{SpotID: "first"})
	if loaded := model.InitialFetchProgress(); loaded != 1 {
		t.Fatalf("progress after one forecast = %d, want 1", loaded)
	}

	model, _ = model.Update(ForecastLoadedMsg{SpotID: "second"})
	if loaded := model.InitialFetchProgress(); loaded != 2 {
		t.Fatalf("completed fetch progress = %d, want 2", loaded)
	}
}

func TestRImmediatelyRefreshesOnlyDashboardForecasts(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	updatedAt := now.Add(-2 * time.Hour)
	spots := []surf.Spot{
		{ID: "first", Name: "First"},
		{ID: "second", Name: "Second"},
	}
	cache := &fakeForecastCache{entries: map[string]surf.ForecastCacheEntry{}}
	for _, spot := range spots {
		cache.entries[spot.ID] = surf.ForecastCacheEntry{
			SpotID: spot.ID,
			Forecast: surf.Forecast{SpotID: spot.ID, Slots: []surf.ForecastSlot{{
				Rating: "Cached " + spot.Name,
			}}},
			ForecastUpdatedAt: updatedAt,
			Details:           surf.ForecastDetails{SpotID: spot.ID, Units: surf.ForecastUnits{WindSpeed: "cached"}},
			DetailsUpdatedAt:  updatedAt,
		}
	}
	provider := &fakeForecastProvider{
		forecast: surf.Forecast{Slots: []surf.ForecastSlot{{Rating: "Fresh"}}},
		details:  surf.ForecastDetails{Units: surf.ForecastUnits{WindSpeed: "KTS"}},
	}
	model := New(provider, cache, spots, nil)
	model.now = func() time.Time { return now }
	model.selectedIndex = 1
	model, cmd := model.Update(dashboardKey('r'))
	if cmd == nil {
		t.Fatal("r did not immediately start the dashboard refresh")
	}
	if model.selectedIndex != 1 {
		t.Fatalf("refresh changed selection to %d, want 1", model.selectedIndex)
	}
	for _, spot := range spots {
		forecast := model.forecasts[spot.ID]
		details := model.details[spot.ID]
		if !forecast.loading || forecast.err != nil || forecast.forecast.Slots[0].Rating != "Cached "+spot.Name {
			t.Fatalf("refresh cleared cached forecast for %s: %+v", spot.ID, forecast)
		}
		if details.loading || details.err != nil || details.details.Units.WindSpeed != "cached" {
			t.Fatalf("dashboard refresh changed cached details for %s: %+v", spot.ID, details)
		}
		nameLine := ansi.Strip(model.spotNameLine(spot.Name, 80, forecast, true))
		if !strings.Contains(nameLine, ansi.Strip(model.refreshSpinner.View())+" updated 2h ago") {
			t.Fatalf("refresh spinner is not immediately left of cached age for %s: %q", spot.ID, nameLine)
		}
	}
	model, duplicate := model.Update(dashboardKey('r'))
	if duplicate != nil {
		t.Fatal("in-flight refresh started duplicate requests")
	}

	messages := commandMessages(t, cmd)
	refreshMessages := make([]tea.Msg, 0, len(spots))
	var spinnerMessage spinner.TickMsg
	for _, message := range messages {
		switch message := message.(type) {
		case ForecastLoadedMsg:
			refreshMessages = append(refreshMessages, message)
		case ForecastDetailsLoadedMsg:
			t.Fatal("dashboard refresh unexpectedly fetched forecast details")
		case spinner.TickMsg:
			spinnerMessage = message
		}
	}
	if len(refreshMessages) != len(spots) {
		t.Fatalf("refresh messages = %d, want %d", len(refreshMessages), len(spots))
	}
	if spinnerMessage.ID == 0 {
		t.Fatal("refresh did not start spinner animation")
	}
	model, nextSpinnerTick := model.Update(spinnerMessage)
	if nextSpinnerTick == nil {
		t.Fatal("active refresh spinner did not schedule its next frame")
	}
	if got := strings.Join(provider.spotIDs, ","); got != "first,second" {
		t.Fatalf("forecast refresh spot IDs = %q, want first,second", got)
	}
	if len(provider.detailSpotIDs) != 0 {
		t.Fatalf("dashboard refresh fetched details for %v", provider.detailSpotIDs)
	}
	for _, message := range refreshMessages {
		model, _ = model.Update(message)
	}
	if model.hasActiveFetches() {
		t.Fatal("completed refresh left pending spinner state")
	}
	for _, spot := range spots {
		forecast := model.forecasts[spot.ID]
		details := model.details[spot.ID]
		if forecast.loading || forecast.err != nil || forecast.forecast.Slots[0].Rating != "Fresh" || forecast.updatedAt != now {
			t.Fatalf("refreshed forecast for %s = %+v", spot.ID, forecast)
		}
		if details.loading || details.err != nil || details.details.Units.WindSpeed != "cached" || details.updatedAt != updatedAt {
			t.Fatalf("dashboard refresh changed details for %s: %+v", spot.ID, details)
		}
		nameLine := ansi.Strip(model.spotNameLine(spot.Name, 80, forecast, model.spotFetching(spot.ID)))
		if strings.Contains(nameLine, ansi.Strip(model.refreshSpinner.View())+" ") {
			t.Fatalf("completed refresh still shows spinner for %s: %q", spot.ID, nameLine)
		}
	}
}

func TestManualRefreshLimitsConcurrentLocations(t *testing.T) {
	t.Parallel()

	spots := []surf.Spot{
		{ID: "first"}, {ID: "second"}, {ID: "third"}, {ID: "fourth"}, {ID: "fifth"},
	}
	cache := &fakeForecastCache{entries: map[string]surf.ForecastCacheEntry{}}
	for _, spot := range spots {
		cache.entries[spot.ID] = surf.ForecastCacheEntry{
			SpotID:            spot.ID,
			Forecast:          surf.Forecast{SpotID: spot.ID, Slots: []surf.ForecastSlot{{Rating: "Fair"}}},
			ForecastUpdatedAt: time.Now(),
		}
	}
	provider := &fakeForecastProvider{forecast: surf.Forecast{Slots: []surf.ForecastSlot{{Rating: "Good"}}}}
	model := New(provider, cache, spots, nil)

	model, cmd := model.Update(dashboardKey('r'))
	if cmd == nil {
		t.Fatal("refresh did not start")
	}
	if active, queued := forecastWorkCounts(model); active != 2 || queued != 3 {
		t.Fatalf("initial refresh work = %d active, %d queued; want 2 active, 3 queued", active, queued)
	}

	messages := commandMessages(t, cmd)
	var firstResult ForecastLoadedMsg
	for _, message := range messages {
		if result, ok := message.(ForecastLoadedMsg); ok {
			firstResult = result
			break
		}
	}
	if firstResult.SpotID == "" {
		t.Fatal("initial refresh commands returned no forecast")
	}
	model, next := model.Update(firstResult)
	if next == nil {
		t.Fatal("finishing one location did not start the next queued location")
	}
	if active, queued := forecastWorkCounts(model); active != 2 || queued != 2 {
		t.Fatalf("continued refresh work = %d active, %d queued; want 2 active, 2 queued", active, queued)
	}
	if nextMessages := commandMessages(t, next); len(nextMessages) != 1 {
		t.Fatalf("next queue step produced %d messages, want 1", len(nextMessages))
	}
}

func forecastWorkCounts(model Model) (active, queued int) {
	for _, state := range model.forecasts {
		if state.loading {
			active++
		}
		if state.queued {
			queued++
		}
	}
	return active, queued
}

func TestRefreshKeepsPreviousAgeUntilSummaryFinishes(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	updatedAt := now.Add(-2 * time.Hour)
	cache := &fakeForecastCache{entries: map[string]surf.ForecastCacheEntry{
		"honolua": {
			SpotID:            "honolua",
			Forecast:          surf.Forecast{SpotID: "honolua", Slots: []surf.ForecastSlot{{Rating: "Fair"}}},
			ForecastUpdatedAt: updatedAt,
			Details:           surf.ForecastDetails{SpotID: "honolua"},
			DetailsUpdatedAt:  updatedAt,
		},
	}}
	model := New(
		&fakeForecastProvider{},
		cache,
		[]surf.Spot{{ID: "honolua", Name: "Honolua Bay"}},
		nil,
	)
	model.now = func() time.Time { return now }
	model, cmd := model.Update(dashboardKey('r'))
	if cmd == nil || !model.spotFetching("honolua") {
		t.Fatal("refresh did not immediately start the summary request")
	}
	nameLine := ansi.Strip(model.spotNameLine(
		"Honolua Bay",
		80,
		model.forecasts["honolua"],
		model.spotFetching("honolua"),
	))
	if !strings.Contains(nameLine, ansi.Strip(model.refreshSpinner.View())+" updated 2h ago") {
		t.Fatalf("active refresh did not preserve previous freshness age: %q", nameLine)
	}

	model, _ = model.Update(ForecastLoadedMsg{
		SpotID:   "honolua",
		Forecast: surf.Forecast{SpotID: "honolua", Slots: []surf.ForecastSlot{{Rating: "Good"}}},
	})
	if model.spotFetching("honolua") {
		t.Fatal("spinner remained after the summary request completed")
	}
	nameLine = ansi.Strip(model.spotNameLine("Honolua Bay", 80, model.forecasts["honolua"], false))
	if !strings.HasSuffix(nameLine, "updated now ") {
		t.Fatalf("completed refresh freshness = %q, want updated now", nameLine)
	}
	if strings.Contains(nameLine, ansi.Strip(model.refreshSpinner.View())+" ") {
		t.Fatalf("completed refresh still showed spinner: %q", nameLine)
	}
}

func TestFailedRefreshKeepsCachedForecastAndShowsAge(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 22, 12, 43, 0, 0, time.UTC)
	updatedAt := now.Add(-90 * time.Minute)
	cache := &fakeForecastCache{entries: map[string]surf.ForecastCacheEntry{
		"honolua": {
			SpotID: "honolua",
			Forecast: surf.Forecast{SpotID: "honolua", Slots: []surf.ForecastSlot{{
				Timestamp:  time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC),
				Rating:     "Fair",
				SurfHeight: surf.SurfHeight{Min: 2, Max: 3},
			}}},
			ForecastUpdatedAt: updatedAt,
		},
	}}
	model := New(nil, cache, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model.now = func() time.Time { return now }
	model, _ = model.Update(ForecastLoadedMsg{
		SpotID: "honolua",
		Err:    errors.New("Surfline wave forecast returned 403: <html>blocked</html>"),
	})

	state := model.forecasts["honolua"]
	if !state.usable() || state.forecast.Slots[0].Rating != "Fair" || state.updatedAt != updatedAt {
		t.Fatalf("failed refresh discarded cached state: %+v", state)
	}
	card := model.spotCard(model.spots[0], 10, false)
	plain := ansi.Strip(card)
	if !strings.Contains(plain, "Fair") || !strings.Contains(plain, "updated 1h ago") {
		t.Fatalf("cached fallback card is missing forecast or age:\n%s", plain)
	}
	if strings.Contains(plain, "Last updated") {
		t.Fatalf("cached fallback card includes the removed last-updated prefix:\n%s", plain)
	}
	if strings.Contains(plain, "403") || strings.Contains(plain, "<html>") {
		t.Fatalf("cached fallback card leaked the raw provider error:\n%s", plain)
	}
	styledAge := ui.DashboardSubtitleStyle.Render("updated 1h ago")
	if !strings.Contains(card, styledAge) {
		t.Fatal("cached age does not use the dashboard subtitle color")
	}
	nameLine := ansi.Strip(model.spotNameLine("Honolua Bay", 80, state, false))
	if !strings.HasSuffix(nameLine, "updated 1h ago ") {
		t.Fatalf("cached age does not have right-border spacing: %q", nameLine)
	}
}

func TestManualRefreshFailureShowsRedDotUntilSuccessfulRefresh(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	updatedAt := now.Add(-2 * time.Hour)
	cache := &fakeForecastCache{entries: map[string]surf.ForecastCacheEntry{
		"honolua": {
			SpotID: "honolua",
			Forecast: surf.Forecast{SpotID: "honolua", Slots: []surf.ForecastSlot{{
				Timestamp: now,
				Rating:    "Fair",
			}}},
			ForecastUpdatedAt: updatedAt,
			Details:           surf.ForecastDetails{SpotID: "honolua"},
			DetailsUpdatedAt:  updatedAt,
		},
	}}
	model := New(
		&fakeForecastProvider{},
		cache,
		[]surf.Spot{{ID: "honolua", Name: "Honolua Bay"}},
		nil,
	)
	model.now = func() time.Time { return now }

	model, _ = model.Update(dashboardKey('r'))
	model, _ = model.Update(ForecastLoadedMsg{
		SpotID: "honolua",
		Err:    errors.New("Surfline wave forecast returned 403"),
	})

	state := model.forecasts["honolua"]
	if !state.refreshFailed || state.manualRefresh || model.spotFetching("honolua") {
		t.Fatalf("completed failed refresh state = %+v", state)
	}
	nameLine := model.spotNameLine("Honolua Bay", 80, state, false)
	if !strings.Contains(ansi.Strip(nameLine), "● couldn’t update · 2h old") {
		t.Fatalf("failed refresh does not explain the failure and cached age: %q", ansi.Strip(nameLine))
	}
	if !strings.Contains(nameLine, "\x1b[31;1m●") {
		t.Fatalf("failed refresh dot does not use the error color: %q", nameLine)
	}

	model, _ = model.Update(dashboardKey('r'))
	model, _ = model.Update(ForecastLoadedMsg{
		SpotID:   "honolua",
		Forecast: surf.Forecast{SpotID: "honolua", Slots: []surf.ForecastSlot{{Timestamp: now, Rating: "Good"}}},
	})

	state = model.forecasts["honolua"]
	nameLine = model.spotNameLine("Honolua Bay", 80, state, false)
	if state.refreshFailed || strings.Contains(ansi.Strip(nameLine), "●") {
		t.Fatalf("successful refresh did not clear the failure dot: %q", ansi.Strip(nameLine))
	}
	if !strings.Contains(ansi.Strip(nameLine), "updated now") {
		t.Fatalf("successful retry freshness = %q, want updated now", ansi.Strip(nameLine))
	}
}

func TestCachedForecastDoesNotStartBackgroundRefresh(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	updatedAt := now.Add(-2 * time.Hour)
	cache := &fakeForecastCache{entries: map[string]surf.ForecastCacheEntry{
		"honolua": {
			SpotID: "honolua",
			Forecast: surf.Forecast{SpotID: "honolua", Slots: []surf.ForecastSlot{{
				Timestamp: time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC),
				Rating:    "Fair",
			}}},
			ForecastUpdatedAt: updatedAt,
		},
	}}
	model := New(&fakeForecastProvider{}, cache, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model.now = func() time.Time { return now }

	plain := ansi.Strip(model.spotCard(model.spots[0], 10, false))
	if !strings.Contains(plain, "Fair") || !strings.Contains(plain, "updated 2h ago") {
		t.Fatalf("cached startup does not identify cached data and its age:\n%s", plain)
	}
	if strings.Contains(plain, ansi.Strip(model.refreshSpinner.View())+" updated 2h ago") {
		t.Fatalf("cached startup unexpectedly shows a refresh spinner:\n%s", plain)
	}
	if strings.Contains(plain, "Last updated") {
		t.Fatalf("cached startup includes the removed last-updated prefix:\n%s", plain)
	}
	if model.Init() != nil {
		t.Fatal("cached startup unexpectedly produced forecast commands")
	}
}

func TestSuccessfulRefreshReplacesAndPersistsCache(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 22, 12, 43, 0, 0, time.UTC)
	cache := &fakeForecastCache{entries: map[string]surf.ForecastCacheEntry{}}
	model := New(nil, cache, []surf.Spot{{ID: "honolua"}}, nil)
	model.now = func() time.Time { return now }
	forecast := surf.Forecast{SpotID: "honolua", Slots: []surf.ForecastSlot{{Rating: "Good"}}}
	model, _ = model.Update(ForecastLoadedMsg{SpotID: "honolua", Forecast: forecast})
	details := surf.ForecastDetails{SpotID: "honolua", Units: surf.ForecastUnits{WindSpeed: "KTS"}}
	model, _ = model.Update(ForecastDetailsLoadedMsg{SpotID: "honolua", Details: details})

	if len(cache.saved) != 2 {
		t.Fatalf("cache saves = %d, want one per successful response", len(cache.saved))
	}
	got := cache.saved[len(cache.saved)-1]
	if got.Forecast.Slots[0].Rating != "Good" || got.Details.Units.WindSpeed != "KTS" ||
		got.ForecastUpdatedAt != now || got.DetailsUpdatedAt != now {
		t.Fatalf("persisted cache entry = %+v", got)
	}
	nameLine := ansi.Strip(model.spotNameLine("Honolua Bay", 80, model.forecasts["honolua"], false))
	if !strings.HasSuffix(nameLine, "updated now ") {
		t.Fatalf("successful fetch freshness = %q, want updated now", nameLine)
	}
}

func TestForecastAgeUsesMinutesHoursAndDays(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		age  time.Duration
		want string
	}{
		{age: 0, want: "1m ago"},
		{age: 30 * time.Second, want: "1m ago"},
		{age: time.Minute, want: "1m ago"},
		{age: 59 * time.Minute, want: "59m ago"},
		{age: time.Hour, want: "1h ago"},
		{age: 23 * time.Hour, want: "23h ago"},
		{age: 24 * time.Hour, want: "1d ago"},
		{age: 49 * time.Hour, want: "2d ago"},
	}
	for _, test := range tests {
		if got := formatForecastAge(now, now.Add(-test.age)); got != test.want {
			t.Errorf("formatForecastAge(%v) = %q, want %q", test.age, got, test.want)
		}
	}
}

func TestViewShowsTodayThroughNextMidnight(t *testing.T) {
	t.Parallel()

	offset := -10 * time.Hour
	now := time.Date(2026, time.July, 21, 16, 45, 0, 0, time.UTC) // 6:45am at the spot.
	forecast := surf.Forecast{SpotID: "honolua", UTCOffset: offset}
	for hour := 0; hour <= 24; hour += 3 {
		localWallTime := time.Date(2026, time.July, 21, hour, 0, 0, 0, time.UTC)
		forecast.Slots = append(forecast.Slots, surf.ForecastSlot{
			Timestamp: localWallTime.Add(-offset),
			Rating:    "Poor",
			SurfHeight: surf.SurfHeight{
				Min:           1,
				Max:           2,
				HumanRelation: "Knee to thigh",
			},
		})
	}
	forecast.Slots = append(forecast.Slots, surf.ForecastSlot{
		Timestamp: time.Date(2026, time.July, 22, 3, 0, 0, 0, time.UTC).Add(-offset),
		Rating:    "Good",
	})

	model := New(nil, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model.forecasts["honolua"] = forecastState{forecast: forecast}
	model.now = func() time.Time { return now }
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	rendered := model.View()
	plain := ansi.Strip(rendered)
	for _, want := range []string{"Honolua Bay", "12a", "6a", "12p", "1–2′"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("view does not contain %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "Good") {
		t.Fatalf("view included a slot after next midnight:\n%s", plain)
	}
	if count := strings.Count(plain, "Poor"); count != 9 {
		t.Fatalf("Poor slot count = %d, want 9:\n%s", count, plain)
	}
	if count := strings.Count(plain, "12a"); count != 2 {
		t.Fatalf("12a header count = %d, want start and end only:\n%s", count, plain)
	}
	card := model.spotCard(model.spots[0], 7, false)
	if strings.Contains(card, "\x1b[48") {
		t.Fatal("forecast card still uses a current-time background highlight")
	}
	currentHour := ui.DashboardCurrentHourStyle.Width(7).MaxWidth(7).Render("3p")
	if !strings.Contains(rendered, currentHour) {
		t.Fatal("time header does not highlight the viewer's current 3pm slot")
	}
	if !strings.Contains(plain, "now") {
		t.Fatal("time header does not label the current slot as now")
	}
}

func TestDashboardStartsWithForecastDatesAndHours(t *testing.T) {
	t.Parallel()

	const width = 80
	model := New(nil, nil, nil, nil)
	model.now = func() time.Time {
		return time.Date(2026, time.July, 22, 15, 0, 0, 0, time.Local)
	}

	header := ansi.Strip(model.dashboardHeader(width))
	lines := strings.Split(header, "\n")
	if !strings.Contains(lines[0], "7/22") || !strings.Contains(lines[0], "7/23") {
		t.Fatalf("forecast boundary dates are not above the hours: %q", lines[0])
	}
	if !strings.Contains(lines[1], "12a") || !strings.Contains(lines[1], "3a") {
		t.Fatalf("forecast hours are not below the dates: %q", lines[1])
	}
	if !strings.Contains(lines[2], "now") {
		t.Fatalf("current-time label is not below the forecast hours: %q", lines[2])
	}
	for _, removed := range []string{"Surfista", "Today's surf conditions"} {
		if strings.Contains(header, removed) {
			t.Fatalf("dashboard header still contains removed top-bar text %q:\n%s", removed, header)
		}
	}
}

func TestDashboardShowsSunlightForSelectedLocationAndDetails(t *testing.T) {
	t.Parallel()

	const width = 80
	now := time.Date(2026, time.July, 23, 2, 0, 0, 0, time.UTC)
	firstSpot := surf.Spot{ID: "first", Name: "First Spot"}
	detailsSpot := surf.Spot{ID: "honolua", Name: "Honolua Bay"}
	model := New(nil, nil, []surf.Spot{firstSpot, detailsSpot}, nil)
	model.now = func() time.Time { return now }
	model.forecasts[firstSpot.ID] = forecastState{forecast: surf.Forecast{
		SpotID: firstSpot.ID,
		Slots:  []surf.ForecastSlot{{Timestamp: time.Date(2026, time.July, 23, 6, 0, 0, 0, time.UTC)}},
	}}
	model.details[firstSpot.ID] = forecastDetailsState{details: surf.ForecastDetails{
		SpotID: firstSpot.ID,
		Sunlight: []surf.SunlightDay{{
			Sunrise: time.Date(2026, time.July, 23, 4, 44, 0, 0, time.UTC),
			Sunset:  time.Date(2026, time.July, 23, 21, 1, 0, 0, time.UTC),
		}},
	}}
	model.forecasts[detailsSpot.ID] = forecastState{forecast: surf.Forecast{
		SpotID:    detailsSpot.ID,
		UTCOffset: -10 * time.Hour,
		Slots: []surf.ForecastSlot{
			{Timestamp: time.Date(2026, time.July, 22, 16, 0, 0, 0, time.UTC)},
			{Timestamp: time.Date(2026, time.July, 23, 16, 0, 0, 0, time.UTC)},
		},
	}}
	model.details[detailsSpot.ID] = forecastDetailsState{details: surf.ForecastDetails{
		SpotID: detailsSpot.ID,
		Sunlight: []surf.SunlightDay{
			{
				Sunrise: time.Date(2026, time.July, 22, 16, 30, 0, 0, time.UTC),
				Sunset:  time.Date(2026, time.July, 23, 6, 12, 0, 0, time.UTC),
			},
			{
				Sunrise: time.Date(2026, time.July, 23, 16, 31, 0, 0, time.UTC),
				Sunset:  time.Date(2026, time.July, 24, 6, 11, 0, 0, time.UTC),
			},
		},
	}}

	dashboardAnnotation := strings.Split(ansi.Strip(model.forecastHeader(width)), "\n")[0]
	if !strings.Contains(dashboardAnnotation, "7/23") || !strings.Contains(dashboardAnnotation, "7/24") {
		t.Fatalf("dashboard annotation row is missing its dates:\n%s", dashboardAnnotation)
	}
	for _, hidden := range []string{"↑4:44a", "↓9:01p", "↑6:30a", "↓8:12p"} {
		if strings.Contains(dashboardAnnotation, hidden) {
			t.Fatalf("unselected dashboard shows location-specific sunlight marker %q:\n%s", hidden, dashboardAnnotation)
		}
	}

	model.selectedIndex = 0
	firstAnnotation := strings.Split(ansi.Strip(model.forecastHeader(width)), "\n")[0]
	if !strings.Contains(firstAnnotation, "↑4:44a") || !strings.Contains(firstAnnotation, "↓9:01p") ||
		strings.Contains(firstAnnotation, "↑6:30a") || strings.Contains(firstAnnotation, "↓8:12p") {
		t.Fatalf("first selected location does not own the sunlight markers:\n%s", firstAnnotation)
	}

	model.selectedIndex = 1
	selectedHeader := model.forecastHeader(width)
	selectedAnnotation := strings.Split(ansi.Strip(selectedHeader), "\n")[0]
	if !strings.Contains(selectedAnnotation, "7/22") || !strings.Contains(selectedAnnotation, "7/23") ||
		!strings.Contains(selectedAnnotation, "↑6:30a") || !strings.Contains(selectedAnnotation, "↓8:12p") {
		t.Fatalf("selected dashboard location is missing its local dates or daylight times:\n%s", selectedAnnotation)
	}
	if strings.Contains(selectedAnnotation, "↑4:44a") || strings.Contains(selectedAnnotation, "↓9:01p") {
		t.Fatalf("selected dashboard location uses another location's daylight times:\n%s", selectedAnnotation)
	}

	model.selectedIndex = 0
	model.detailsOpen = true
	model.detailsSpot = detailsSpot
	header := model.forecastHeader(width)
	annotation := strings.Split(ansi.Strip(header), "\n")[0]
	if !strings.Contains(annotation, "7/22") || !strings.Contains(annotation, "7/23") ||
		!strings.Contains(annotation, "↑6:30a") || !strings.Contains(annotation, "↓8:12p") {
		t.Fatalf("details annotation row is missing selected-location dates or daylight times:\n%s", annotation)
	}
	if strings.Contains(annotation, "↑4:44a") || strings.Contains(annotation, "↓9:01p") {
		t.Fatalf("details annotation row did not override the dashboard selection:\n%s", annotation)
	}

	slotWidth := dashboardSlotWidth(width)
	cardWidth := slotWidth*len(dashboardHours) + gridBorderWidth
	headerOffset := (width - cardWidth) / 2
	if got, want := sunlightPivotColumn(annotation, "↑6:30a"), headerOffset+dashboardTimePosition(model.details[detailsSpot.ID].details.Sunlight[0].Sunrise, -10*time.Hour, slotWidth); got != want {
		t.Fatalf("sunrise time pivot column = %d, want exact-time column %d:\n%s", got, want, annotation)
	}
	if got, want := sunlightPivotColumn(annotation, "↓8:12p"), headerOffset+dashboardTimePosition(model.details[detailsSpot.ID].details.Sunlight[0].Sunset, -10*time.Hour, slotWidth); got != want {
		t.Fatalf("sunset time pivot column = %d, want exact-time column %d:\n%s", got, want, annotation)
	}
	styledSunrise := ui.DashboardSubtitleStyle.Render("↑6:30a")
	if !strings.Contains(header, styledSunrise) {
		t.Fatal("sunrise marker does not use the same style as the dashboard dates")
	}

	model.forecastDayOffset = 1
	nextAnnotation := strings.Split(ansi.Strip(model.forecastHeader(width)), "\n")[0]
	if !strings.Contains(nextAnnotation, "7/23") || !strings.Contains(nextAnnotation, "7/24") ||
		!strings.Contains(nextAnnotation, "↑6:31a") || !strings.Contains(nextAnnotation, "↓8:11p") || strings.Contains(nextAnnotation, "↑6:30a") {
		t.Fatalf("next-day details header does not use the selected location's next-day data:\n%s", nextAnnotation)
	}
}

func TestDashboardDayNavigationUsesAvailableForecastDates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 22, 15, 0, 0, 0, time.UTC)
	forecast := surf.Forecast{SpotID: "honolua"}
	for dayOffset, rating := range []string{"Poor", "Good", "Epic"} {
		for hour := 0; hour < 24; hour += 3 {
			forecast.Slots = append(forecast.Slots, surf.ForecastSlot{
				Timestamp:  time.Date(2026, time.July, 22+dayOffset, hour, 0, 0, 0, time.UTC),
				Rating:     rating,
				SurfHeight: surf.SurfHeight{Min: float64(dayOffset + 1), Max: float64(dayOffset + 2)},
			})
		}
	}

	spot := surf.Spot{ID: "honolua", Name: "Honolua Bay"}
	model := New(nil, nil, []surf.Spot{spot}, nil)
	model.now = func() time.Time { return now }
	model.forecasts[spot.ID] = forecastState{forecast: forecast}

	model, _ = model.Update(dashboardKey('l'))
	if model.forecastDayOffset != 1 {
		t.Fatalf("day offset after l = %d, want 1", model.forecastDayOffset)
	}
	header := ansi.Strip(model.forecastHeader(80))
	card := ansi.Strip(model.spotCard(spot, 7, false))
	if !strings.Contains(header, "7/23") || !strings.Contains(header, "7/24") {
		t.Fatalf("next-day header does not show 7/23 -> 7/24:\n%s", header)
	}
	if strings.Contains(header, "now") {
		t.Fatalf("future-day header still marks a current hour:\n%s", header)
	}
	if !strings.Contains(card, "Good") || strings.Contains(card, "Poor") {
		t.Fatalf("next-day card does not use the next forecast day:\n%s", card)
	}

	model, _ = model.Update(dashboardKey('l'))
	model, _ = model.Update(dashboardKey('l'))
	if model.forecastDayOffset != 2 {
		t.Fatalf("day offset moved past available data: %d", model.forecastDayOffset)
	}
	if header := ansi.Strip(model.forecastHeader(80)); !strings.Contains(header, "7/24") || !strings.Contains(header, "7/25") {
		t.Fatalf("last-day header does not show 7/24 -> 7/25:\n%s", header)
	}

	model, _ = model.Update(dashboardKey('h'))
	model, _ = model.Update(dashboardKey('h'))
	model, _ = model.Update(dashboardKey('h'))
	if model.forecastDayOffset != 0 {
		t.Fatalf("day offset moved before today: %d", model.forecastDayOffset)
	}
	if header := ansi.Strip(model.forecastHeader(80)); !strings.Contains(header, "now") {
		t.Fatalf("today header did not restore the current-hour marker:\n%s", header)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if model.forecastDayOffset != 1 {
		t.Fatalf("day offset after right arrow = %d, want 1", model.forecastDayOffset)
	}
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if model.forecastDayOffset != 0 {
		t.Fatalf("day offset after left arrow = %d, want 0", model.forecastDayOffset)
	}
}

func TestDashboardDatesUseForecastLocalDay(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 23, 2, 0, 0, 0, time.UTC)
	spot := surf.Spot{ID: "sunzal", Name: "Sunzal"}
	model := New(nil, nil, []surf.Spot{spot}, nil)
	model.now = func() time.Time { return now }
	model.forecasts[spot.ID] = forecastState{forecast: surf.Forecast{
		SpotID:    spot.ID,
		UTCOffset: -6 * time.Hour,
		Slots: []surf.ForecastSlot{{
			Timestamp: time.Date(2026, time.July, 22, 12, 0, 0, 0, time.FixedZone("spot", -6*60*60)),
		}},
	}}

	header := ansi.Strip(model.forecastHeader(80))
	if !strings.Contains(header, "7/22") || !strings.Contains(header, "7/23") {
		t.Fatalf("header dates do not use the forecast's local day:\n%s", header)
	}
	if strings.Contains(header, "7/24") {
		t.Fatalf("header dates incorrectly use the viewer's UTC day:\n%s", header)
	}
}

func TestHeightUsesFeetInsteadOfBodyRelation(t *testing.T) {
	t.Parallel()

	height := surf.SurfHeight{Min: 1, Max: 2, Plus: true, HumanRelation: "Knee to thigh"}
	if got := formatHeight(height); got != "1–2′+" {
		t.Fatalf("height = %q, want %q", got, "1–2′+")
	}
}

func TestForecastCardColorsRatingsByCondition(t *testing.T) {
	t.Parallel()

	ratings := []string{"Very Poor", "Poor", "Poor to Fair", "Fair", "Fair to Good", "Good", "Very Good", "Epic"}
	forecast := surf.Forecast{SpotID: "honolua"}
	for index, rating := range ratings {
		forecast.Slots = append(forecast.Slots, surf.ForecastSlot{
			Timestamp:  time.Date(2026, time.July, 21, index*3, 0, 0, 0, time.UTC),
			Rating:     rating,
			SurfHeight: surf.SurfHeight{Min: 1, Max: 2},
		})
	}

	spot := surf.Spot{ID: "honolua", Name: "Honolua Bay"}
	model := New(nil, nil, []surf.Spot{spot}, nil)
	model.forecasts[spot.ID] = forecastState{forecast: forecast}
	model.now = func() time.Time { return time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC) }
	card := model.spotCard(spot, 10, false)
	for _, rating := range ratings {
		compact := compactRating(rating, 10)
		colored := ui.DashboardRating(compact, rating)
		if !strings.Contains(card, colored) {
			t.Fatalf("forecast card does not color %q:\n%s", rating, ansi.Strip(card))
		}
	}
}

func TestNarrowViewKeepsForecastOnSharedAxis(t *testing.T) {
	t.Parallel()

	offset := -10 * time.Hour
	now := time.Date(2026, time.July, 21, 16, 0, 0, 0, time.UTC)
	forecast := surf.Forecast{SpotID: "honolua", UTCOffset: offset}
	for hour := 0; hour <= 24; hour += 3 {
		forecast.Slots = append(forecast.Slots, surf.ForecastSlot{
			Timestamp:  time.Date(2026, time.July, 21, hour, 0, 0, 0, time.UTC).Add(-offset),
			Rating:     "Poor to Fair",
			SurfHeight: surf.SurfHeight{Min: 1, Max: 2},
		})
	}

	model := New(nil, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model.forecasts["honolua"] = forecastState{forecast: forecast}
	model.now = func() time.Time { return now }
	model, _ = model.Update(tea.WindowSizeMsg{Width: 50, Height: 20})

	plain := ansi.Strip(model.View())
	nameCenteredInBox := false
	for _, line := range strings.Split(plain, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "│") && strings.HasSuffix(trimmed, "│") && strings.Contains(trimmed, "Honolua Bay") {
			nameCenteredInBox = true
			break
		}
	}
	if !nameCenteredInBox {
		t.Fatalf("narrow view did not center the spot name in its box:\n%s", plain)
	}
	if count := strings.Count(plain, "P–F"); count != 9 {
		t.Fatalf("compact rating count = %d, want 9:\n%s", count, plain)
	}
	for _, line := range strings.Split(plain, "\n") {
		if width := ansi.StringWidth(line); width > 50 {
			t.Fatalf("line width = %d, want at most 50: %q", width, line)
		}
	}
}

func TestTimeHeaderIsUnboxedAboveLocationCards(t *testing.T) {
	t.Parallel()

	model := New(nil, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model.forecasts["honolua"] = forecastState{forecast: surf.Forecast{SpotID: "honolua"}}
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	plain := ansi.Strip(model.View())

	lines := strings.Split(plain, "\n")
	headerIndex := -1
	boxIndex := -1
	for index, line := range lines {
		if strings.Contains(line, "12a") && strings.Contains(line, "3a") {
			headerIndex = index
			if strings.ContainsAny(line, "│┌┐╭╮") {
				t.Fatalf("time header has a border: %q", line)
			}
		}
		if strings.Contains(line, "╭") {
			boxIndex = index
			break
		}
	}
	if headerIndex < 0 || boxIndex <= headerIndex {
		t.Fatalf("time header index = %d, box index = %d:\n%s", headerIndex, boxIndex, plain)
	}
}

func TestCurrentSlotUsesViewerLocalTimeAcrossLocations(t *testing.T) {
	t.Parallel()

	viewerTime := time.Date(2026, time.July, 21, 15, 10, 0, 0, time.FixedZone("viewer", -5*60*60))
	if !isCurrentDashboardHour(15, viewerTime) {
		t.Fatal("3pm column was not current at 3:10pm")
	}
	if isCurrentDashboardHour(9, viewerTime) {
		t.Fatal("9am column was current at 3:10pm")
	}
	if isCurrentDashboardHour(24, viewerTime) {
		t.Fatal("next-midnight boundary must not be highlighted during the day")
	}
}

func TestAddStartsForecastForNewFavorite(t *testing.T) {
	t.Parallel()

	provider := &fakeForecastProvider{}
	model := New(provider, nil, nil, nil)
	cmd := model.Add(surf.Spot{ID: "pine-trees", Name: "Pine Trees"})
	if cmd == nil {
		t.Fatal("Add returned no forecast command")
	}
	foundForecast := false
	for _, message := range commandMessages(t, cmd) {
		if loaded, ok := message.(ForecastLoadedMsg); ok {
			foundForecast = true
			if loaded.SpotID != "pine-trees" {
				t.Fatalf("forecast message = %+v", loaded)
			}
		}
	}
	if !foundForecast {
		t.Fatal("Add command did not return a forecast message")
	}
	if len(model.spots) != 1 || !model.forecasts["pine-trees"].loading {
		t.Fatalf("dashboard state = %+v", model)
	}
}

func TestLocationSelectionStartsEmptyAndMovesWithJK(t *testing.T) {
	t.Parallel()

	model := New(nil, nil, []surf.Spot{
		{ID: "first", Name: "First"},
		{ID: "second", Name: "Second"},
		{ID: "third", Name: "Third"},
	}, nil)
	if model.selectedIndex != -1 {
		t.Fatalf("initial selection = %d, want -1", model.selectedIndex)
	}

	model, _ = model.Update(dashboardKey('j'))
	if model.selectedIndex != 0 {
		t.Fatalf("selection after first j = %d, want 0", model.selectedIndex)
	}
	model, _ = model.Update(dashboardKey('j'))
	if model.selectedIndex != 1 {
		t.Fatalf("selection after second j = %d, want 1", model.selectedIndex)
	}
	model, _ = model.Update(dashboardKey('k'))
	if model.selectedIndex != 0 {
		t.Fatalf("selection after k = %d, want 0", model.selectedIndex)
	}
	model, _ = model.Update(dashboardKey('k'))
	if model.selectedIndex != 0 {
		t.Fatalf("selection moved above first location: %d", model.selectedIndex)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if model.selectedIndex != -1 {
		t.Fatalf("selection after Esc = %d, want -1", model.selectedIndex)
	}
	model, _ = model.Update(dashboardKey('k'))
	if model.selectedIndex != 2 {
		t.Fatalf("selection after k from empty = %d, want last location", model.selectedIndex)
	}
}

func TestDashboardHelpShowsOnlyAvailableActions(t *testing.T) {
	t.Parallel()

	model := New(&fakeForecastProvider{}, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model, _ = model.Update(ForecastLoadedMsg{
		SpotID:   "honolua",
		Forecast: surf.Forecast{SpotID: "honolua"},
	})
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	plain := ansi.Strip(model.View())
	for _, want := range []string{"←/→/h/l day", "↑/↓/j/k", "s sort", "r refresh", "/ search", "q quit"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("unselected dashboard help does not contain %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "• enter •") {
		t.Fatalf("unselected dashboard help contains unavailable details action:\n%s", plain)
	}
	if strings.Contains(plain, "s or /") {
		t.Fatalf("unselected dashboard help still offers s as a search shortcut:\n%s", plain)
	}
	for _, unavailable := range []string{"x remove", "• esc •"} {
		if strings.Contains(plain, unavailable) {
			t.Fatalf("unselected dashboard help contains unavailable action %q:\n%s", unavailable, plain)
		}
	}

	model, _ = model.Update(dashboardKey('j'))
	plain = ansi.Strip(model.View())
	for _, want := range []string{"←/→/h/l day", "↑/↓/j/k", "s sort", "r refresh", "enter", "x remove", "esc", "q quit"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("selected dashboard help does not contain %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "/ search") {
		t.Fatalf("selected dashboard help still offers search:\n%s", plain)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	plain = ansi.Strip(model.View())
	if !strings.Contains(plain, "r refresh") || !strings.Contains(plain, "/ search") || strings.Contains(plain, "x remove") || strings.Contains(plain, "• esc •") {
		t.Fatalf("dashboard help did not return to browsing actions after Esc:\n%s", plain)
	}
}

func TestLocationViewportScrollsWhileHeaderAndControlsStayPinned(t *testing.T) {
	t.Parallel()

	model := New(nil, nil, []surf.Spot{
		{ID: "first", Name: "First Location"},
		{ID: "second", Name: "Second Location"},
		{ID: "third", Name: "Third Location"},
		{ID: "fourth", Name: "Fourth Location"},
	}, nil)
	model.now = func() time.Time { return time.Date(2026, time.July, 22, 15, 0, 0, 0, time.UTC) }
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	plain := ansi.Strip(model.View())
	lines := strings.Split(plain, "\n")
	if len(lines) != 24 {
		t.Fatalf("view height = %d, want terminal height 24:\n%s", len(lines), plain)
	}
	if !strings.Contains(lines[0], "7/22") || !strings.Contains(lines[0], "7/23") {
		t.Fatalf("date header is not pinned to the first line: %q", lines[0])
	}
	if !strings.Contains(lines[1], "12a") || !strings.Contains(lines[1], "3a") {
		t.Fatalf("time header is not directly below the dates: %q", lines[1])
	}
	if !strings.Contains(lines[2], "now") {
		t.Fatalf("current-time label is not directly below the time header: %q", lines[2])
	}
	if strings.TrimSpace(lines[3]) != "" {
		t.Fatalf("expected one blank row between the current-time label and sort status: %q", lines[3])
	}
	if !strings.Contains(lines[4], "sorting by: time added") || strings.Contains(lines[4], "s sort") {
		t.Fatalf("sort status is not between the times and locations: %q", lines[4])
	}
	if !strings.Contains(lines[len(lines)-2], "↑/↓/j/k") || !strings.Contains(lines[len(lines)-2], "s sort") || strings.TrimSpace(lines[len(lines)-1]) != "" {
		t.Fatalf("controls are not one row above the bottom: %q / %q", lines[len(lines)-2], lines[len(lines)-1])
	}
	for _, want := range []string{"First Location", "Second Location"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("initial viewport does not contain %q:\n%s", want, plain)
		}
	}
	for _, hidden := range []string{"Third Location", "Fourth Location"} {
		if strings.Contains(plain, hidden) {
			t.Fatalf("initial viewport unexpectedly contains %q:\n%s", hidden, plain)
		}
	}
	if arrowLine := standaloneIndicatorLine(lines, "↓"); arrowLine != len(lines)-3 {
		t.Fatalf("down arrow line = %d, want bottom of location viewport at %d:\n%s", arrowLine, len(lines)-3, plain)
	}
	if standaloneIndicatorLine(lines, "↑") >= 0 {
		t.Fatalf("initial viewport unexpectedly contains an up indicator:\n%s", plain)
	}
	initialFirstCardLine := lineContaining(lines, "╭")

	for range 3 {
		model, _ = model.Update(dashboardKey('j'))
	}
	plain = ansi.Strip(model.View())
	lines = strings.Split(plain, "\n")
	for _, want := range []string{"Second Location", "Third Location"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("scrolled viewport does not contain %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "First Location") || strings.Contains(plain, "Fourth Location") {
		t.Fatalf("scrolled viewport shows a location outside its window:\n%s", plain)
	}
	if arrowLine := standaloneIndicatorLine(lines, "↑"); arrowLine != 5 {
		t.Fatalf("up arrow line = %d, want below the sort status at 5:\n%s", arrowLine, plain)
	}
	upLine := standaloneIndicatorLine(lines, "↑")
	downLine := standaloneIndicatorLine(lines, "↓")
	if downLine < 0 {
		t.Fatalf("scrolled viewport does not contain a down indicator:\n%s", plain)
	}
	firstCardLine := lineContaining(lines, "╭")
	lastCardLine := lastLineContaining(lines, "╯")
	if aboveGap, belowGap := firstCardLine-upLine-1, downLine-lastCardLine-1; aboveGap != belowGap {
		t.Fatalf("location cards are not vertically centered between arrows: above=%d below=%d\n%s", aboveGap, belowGap, plain)
	}
	if firstCardLine != initialFirstCardLine {
		t.Fatalf("cards shifted when the top arrow appeared: initial row=%d scrolled row=%d\n%s", initialFirstCardLine, firstCardLine, plain)
	}
	if !strings.Contains(lines[0], "7/22") || !strings.Contains(lines[1], "12a") || !strings.Contains(lines[4], "sorting by: time added") ||
		!strings.Contains(lines[len(lines)-2], "↑/↓/j/k") || strings.TrimSpace(lines[len(lines)-1]) != "" {
		t.Fatalf("fixed regions moved after scrolling:\n%s", plain)
	}

	model, _ = model.Update(dashboardKey('j'))
	plain = ansi.Strip(model.View())
	lines = strings.Split(plain, "\n")
	if !strings.Contains(plain, "Third Location") || !strings.Contains(plain, "Fourth Location") ||
		standaloneIndicatorLine(lines, "↑") < 0 || standaloneIndicatorLine(lines, "↓") >= 0 {
		t.Fatalf("last viewport does not show the final locations and only the up arrow:\n%s", plain)
	}
	if finalFirstCardLine := lineContaining(lines, "╭"); finalFirstCardLine != initialFirstCardLine {
		t.Fatalf("cards shifted when the bottom arrow disappeared: initial row=%d final row=%d\n%s", initialFirstCardLine, finalFirstCardLine, plain)
	}

	for range 3 {
		model, _ = model.Update(dashboardKey('k'))
	}
	plain = ansi.Strip(model.View())
	lines = strings.Split(plain, "\n")
	if !strings.Contains(plain, "First Location") || standaloneIndicatorLine(lines, "↑") >= 0 || standaloneIndicatorLine(lines, "↓") < 0 {
		t.Fatalf("scrolling back up did not restore the first viewport:\n%s", plain)
	}
}

func TestLocationViewportReflowsAroundSelectionAfterResize(t *testing.T) {
	t.Parallel()

	model := New(nil, nil, []surf.Spot{
		{ID: "first", Name: "First Location"},
		{ID: "second", Name: "Second Location"},
		{ID: "third", Name: "Third Location"},
		{ID: "fourth", Name: "Fourth Location"},
	}, nil)
	model.now = func() time.Time { return time.Date(2026, time.July, 22, 15, 0, 0, 0, time.UTC) }
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	for range 3 {
		model, _ = model.Update(dashboardKey('j'))
	}
	if model.selectedIndex != 2 || model.scrollOffset != 0 {
		t.Fatalf("large viewport state = selection %d offset %d, want selection 2 offset 0", model.selectedIndex, model.scrollOffset)
	}

	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 11})
	plain := ansi.Strip(model.View())
	lines := strings.Split(plain, "\n")
	if len(lines) != 11 {
		t.Fatalf("resized view height = %d, want 11:\n%s", len(lines), plain)
	}
	if model.scrollOffset == 0 || !strings.Contains(plain, "Third Location") {
		t.Fatalf("resize did not scroll the selected third location into view: offset=%d\n%s", model.scrollOffset, plain)
	}
	if !strings.Contains(lines[0], "7/22") || !strings.Contains(lines[1], "12a") || !strings.Contains(lines[2], "now") {
		t.Fatalf("compact layout did not keep forecast time at the top:\n%s", plain)
	}
	if !strings.Contains(lines[len(lines)-2], "↑/↓/j/k") || strings.TrimSpace(lines[len(lines)-1]) != "" {
		t.Fatalf("compact layout did not keep controls at the bottom:\n%s", plain)
	}
	if !strings.Contains(lines[3], "sorting by: time added") || !strings.Contains(lines[len(lines)-2], "s sort") {
		t.Fatalf("compact layout did not separate sort status from its bottom control:\n%s", plain)
	}
}

func lineContaining(lines []string, value string) int {
	for index, line := range lines {
		if strings.Contains(line, value) {
			return index
		}
	}
	return -1
}

func sunlightPivotColumn(line, label string) int {
	index := strings.Index(line, label)
	if index < 0 {
		return -1
	}
	return ansi.StringWidth(line[:index]) + sunlightLabelPivot(label)
}

func standaloneIndicatorLine(lines []string, indicator string) int {
	for index, line := range lines {
		if strings.TrimSpace(line) == indicator {
			return index
		}
	}
	return -1
}

func lastLineContaining(lines []string, value string) int {
	for index := len(lines) - 1; index >= 0; index-- {
		if strings.Contains(lines[index], value) {
			return index
		}
	}
	return -1
}

func TestSelectedLocationUsesWhiteBordersThroughout(t *testing.T) {
	t.Parallel()

	spot := surf.Spot{ID: "honolua", Name: "Honolua Bay"}
	model := New(nil, nil, []surf.Spot{spot}, nil)
	card := model.spotCard(spot, 7, true)
	lines := strings.Split(card, "\n")
	innerWidth := 7*len(dashboardHours) + len(dashboardHours) - 1

	wantTop := ui.DashboardSelectedBorderStyle.Render("╭" + strings.Repeat("─", innerWidth) + "╮")
	wantBottom := ui.DashboardSelectedBorderStyle.Render("╰" + strings.Repeat("─", innerWidth) + "╯")
	if lines[0] != wantTop {
		t.Fatalf("selected top border was not white:\n%s", card)
	}
	if lines[len(lines)-1] != wantBottom {
		t.Fatalf("selected bottom border was not white:\n%s", card)
	}
	wantDivider := ui.DashboardSelectedBorderStyle.Render(ansi.Strip(lines[2]))
	if lines[2] != wantDivider {
		t.Fatalf("selected internal divider was not white:\n%s", card)
	}
	for _, line := range lines[1 : len(lines)-1] {
		plain := ansi.Strip(line)
		if line == ui.DashboardSelectedBorderStyle.Render(plain) {
			continue
		}
		plainLine := []rune(plain)
		leftEdge := ui.DashboardSelectedBorderStyle.Render(string(plainLine[0]))
		rightEdge := ui.DashboardSelectedBorderStyle.Render(string(plainLine[len(plainLine)-1]))
		if !strings.HasPrefix(line, leftEdge) || !strings.HasSuffix(line, rightEdge) {
			t.Fatalf("selected row does not have two white outer edges: %q", line)
		}
	}

	var bottom strings.Builder
	for index := range dashboardHours {
		if index == 0 {
			bottom.WriteString("╰")
		} else {
			bottom.WriteString("┴")
		}
		bottom.WriteString(strings.Repeat("─", 7))
	}
	bottom.WriteString("╯")
	wantSegmentedBorder := ui.DashboardSelectedBorderStyle.Render(bottom.String())
	if got := segmentedBorder("╰", "┴", "╯", 7, true); got != wantSegmentedBorder {
		t.Fatalf("selected forecast grid border was not entirely white: %q", got)
	}

	cells := []string{"rating", "height"}
	wantGridLine := ui.DashboardSelectedBorderStyle.Render("│") + cells[0] +
		ui.DashboardSelectedBorderStyle.Render("│") + cells[1] +
		ui.DashboardSelectedBorderStyle.Render("│")
	if got := segmentedLine(cells, true); got != wantGridLine {
		t.Fatalf("selected forecast grid cells were not outlined in white: %q", got)
	}
}

func dashboardKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}

func commandMessages(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	message := cmd()
	batch, ok := message.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{message}
	}
	messages := make([]tea.Msg, 0, len(batch))
	for _, command := range batch {
		if command != nil {
			messages = append(messages, command())
		}
	}
	return messages
}

type fakeRemover struct {
	removed bool
	err     error
	spotIDs []string
}

func (r *fakeRemover) Remove(spotID string) (bool, error) {
	r.spotIDs = append(r.spotIDs, spotID)
	return r.removed, r.err
}

func TestRemoveConfirmationCanBeCancelled(t *testing.T) {
	t.Parallel()

	remover := &fakeRemover{removed: true}
	model := New(nil, remover, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model, _ = model.Update(dashboardKey('x'))
	if model.confirmRemoval {
		t.Fatal("x opened removal confirmation without a selected location")
	}

	model, _ = model.Update(dashboardKey('j'))
	model, _ = model.Update(dashboardKey('x'))
	if !model.confirmRemoval || model.removalSpot.ID != "honolua" {
		t.Fatalf("removal state = %+v, want Honolua confirmation", model.removalSpot)
	}
	model, _ = model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	plain := ansi.Strip(model.View())
	for _, want := range []string{"Remove Honolua Bay from tracked locations?", "enter remove", "esc cancel"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("confirmation view does not contain %q:\n%s", want, plain)
		}
	}
	if !strings.Contains(model.removalDialog(), ui.DashboardSpotStyle.Render("Honolua Bay")) {
		t.Fatal("confirmation location does not use the dashboard location style")
	}
	if !strings.Contains(model.removalDialog(), ui.SuccessStyle.Render("remove")) {
		t.Fatal("remove action does not use the success style")
	}
	if !strings.Contains(model.removalDialog(), ui.ErrorStyle.Render("cancel")) {
		t.Fatal("cancel action does not use the error style")
	}
	assertDialogTextCentered(t, model.removalDialog(), "Remove Honolua Bay from tracked locations?")
	assertDialogTextCentered(t, model.removalDialog(), "enter remove • esc cancel")
	assertConfirmationDialogCentered(t, model, model.removalDialog())

	model, _ = model.Update(dashboardKey('j'))
	if model.selectedIndex != 0 {
		t.Fatalf("selection moved while confirmation was open: %d", model.selectedIndex)
	}
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if model.confirmRemoval {
		t.Fatal("Esc did not close removal confirmation")
	}
	if model.selectedIndex != 0 || len(model.spots) != 1 {
		t.Fatalf("cancel changed dashboard state: selection=%d spots=%v", model.selectedIndex, model.spots)
	}
	if len(remover.spotIDs) != 0 {
		t.Fatalf("cancel called remover with %v", remover.spotIDs)
	}
}

func assertDialogTextCentered(t *testing.T, dialog, text string) {
	t.Helper()

	for _, line := range strings.Split(ansi.Strip(dialog), "\n") {
		byteIndex := strings.Index(line, text)
		if byteIndex < 0 {
			continue
		}
		left := ansi.StringWidth(line[:byteIndex])
		right := ansi.StringWidth(line) - left - ansi.StringWidth(text)
		if difference := left - right; difference < -1 || difference > 1 {
			t.Fatalf("%q is not centered: left padding=%d right padding=%d", text, left, right)
		}
		return
	}
	t.Fatalf("dialog does not contain %q", text)
}

func assertConfirmationDialogCentered(t *testing.T, model Model, renderedDialog string) {
	t.Helper()

	dialog := ansi.Strip(renderedDialog)
	dialogLines := strings.Split(dialog, "\n")
	viewLines := strings.Split(ansi.Strip(model.View()), "\n")
	wantX := (model.terminalWidth - ansi.StringWidth(dialogLines[0])) / 2
	wantY := (model.terminalHeight - len(dialogLines)) / 2
	if wantY < 0 || wantY >= len(viewLines) {
		t.Fatalf("centered dialog row %d is outside rendered view", wantY)
	}
	byteIndex := strings.Index(viewLines[wantY], dialogLines[0])
	if byteIndex < 0 {
		t.Fatalf("dialog top border is not at centered row %d:\n%s", wantY, ansi.Strip(model.View()))
	}
	if gotX := ansi.StringWidth(viewLines[wantY][:byteIndex]); gotX != wantX {
		t.Fatalf("dialog x = %d, want centered x %d", gotX, wantX)
	}
}

func TestConfirmRemoveDeletesTrackedLocationFromDashboard(t *testing.T) {
	t.Parallel()

	remover := &fakeRemover{removed: true}
	model := New(nil, remover, []surf.Spot{
		{ID: "honolua", Name: "Honolua Bay"},
		{ID: "pine-trees", Name: "Pine Trees"},
	}, nil)
	model, _ = model.Update(dashboardKey('j'))
	model, _ = model.Update(dashboardKey('x'))
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || !model.removing {
		t.Fatal("Enter did not start removal")
	}

	result := cmd()
	message, ok := result.(SpotRemovedMsg)
	if !ok {
		t.Fatalf("remove command returned %T, want SpotRemovedMsg", result)
	}
	model, _ = model.Update(message)
	if len(remover.spotIDs) != 1 || remover.spotIDs[0] != "honolua" {
		t.Fatalf("remover calls = %v, want honolua", remover.spotIDs)
	}
	if len(model.spots) != 1 || model.spots[0].ID != "pine-trees" {
		t.Fatalf("spots after removal = %+v, want only Pine Trees", model.spots)
	}
	if _, exists := model.forecasts["honolua"]; exists {
		t.Fatal("removed spot forecast remains in dashboard")
	}
	if model.selectedIndex != -1 || model.confirmRemoval {
		t.Fatalf("removal did not reset selection/modal: selection=%d confirmation=%v", model.selectedIndex, model.confirmRemoval)
	}
}

func TestRemoveFailureKeepsLocationAndShowsError(t *testing.T) {
	t.Parallel()

	remover := &fakeRemover{err: errors.New("disk unavailable")}
	model := New(nil, remover, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model, _ = model.Update(dashboardKey('j'))
	model, _ = model.Update(dashboardKey('x'))
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	message := cmd().(SpotRemovedMsg)
	model, _ = model.Update(message)

	if len(model.spots) != 1 || !model.confirmRemoval || model.removalErr == nil {
		t.Fatalf("failed removal changed state: spots=%v confirmation=%v err=%v", model.spots, model.confirmRemoval, model.removalErr)
	}
	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "Could not remove: disk unavailable") || !strings.Contains(plain, "enter retry") {
		t.Fatalf("failed removal view does not show retry guidance:\n%s", plain)
	}
}
