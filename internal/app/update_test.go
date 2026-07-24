package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"surfista/internal/screens/dashboard"
	"surfista/internal/surf"
)

type resizeTestSearcher struct{}

func (resizeTestSearcher) SearchSpots(context.Context, string) ([]surf.Spot, error) {
	return nil, nil
}

type resizeTestTracker struct{}

func (resizeTestTracker) Add(surf.Spot) (bool, error) {
	return true, nil
}

func (resizeTestTracker) Remove(string) (bool, error) {
	return true, nil
}

type appTestCacheTracker struct {
	resizeTestTracker
	entries map[string]surf.ForecastCacheEntry
}

func (t appTestCacheTracker) LoadForecastCache() (map[string]surf.ForecastCacheEntry, error) {
	return t.entries, nil
}

func (appTestCacheTracker) SaveForecastCache(surf.ForecastCacheEntry) error {
	return nil
}

func TestWindowSizeReachesSearchBeforeScreenOpens(t *testing.T) {
	t.Parallel()

	model := New(resizeTestSearcher{}, resizeTestTracker{}, nil, nil, nil)
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 34, Height: 20})
	updated := updatedModel.(Model)

	if updated.search.ContentWidth() != 30 {
		t.Fatalf("search content width = %d, want 30", updated.search.ContentWidth())
	}
	if updated.current != homeScreen {
		t.Fatal("resize changed the active screen")
	}
}

func TestInitialForecastsKeepLoadingScreenUntilAllResolve(t *testing.T) {
	t.Parallel()

	model := New(
		resizeTestSearcher{},
		resizeTestTracker{},
		&appTestForecastProvider{},
		[]surf.Spot{{ID: "honolua"}, {ID: "pine-trees"}},
		nil,
	)
	if model.current != loadingScreen {
		t.Fatalf("initial screen = %v, want loading", model.current)
	}

	updatedModel, _ := model.Update(dashboard.ForecastLoadedMsg{SpotID: "honolua"})
	updated := updatedModel.(Model)
	if updated.current != loadingScreen {
		t.Fatalf("screen after first forecast = %v, want loading", updated.current)
	}

	updatedModel, _ = updated.Update(dashboard.ForecastLoadedMsg{
		SpotID: "pine-trees",
		Err:    errors.New("forecast unavailable"),
	})
	updated = updatedModel.(Model)
	if updated.current != homeScreen {
		t.Fatalf("screen after all forecasts = %v, want home", updated.current)
	}
}

func TestInitialLoadingWaitsForPrefetchedForecastDetails(t *testing.T) {
	t.Parallel()

	model := New(
		resizeTestSearcher{},
		resizeTestTracker{},
		&appTestFullForecastProvider{},
		[]surf.Spot{{ID: "honolua"}},
		nil,
	)
	if model.current != loadingScreen || model.initialForecasts != 2 {
		t.Fatalf("initial state = screen %v loads %d, want loading screen with 2 loads", model.current, model.initialForecasts)
	}

	updatedModel, _ := model.Update(dashboard.ForecastLoadedMsg{SpotID: "honolua"})
	updated := updatedModel.(Model)
	if updated.current != loadingScreen {
		t.Fatal("dashboard opened before detail prefetch completed")
	}

	updatedModel, _ = updated.Update(dashboard.ForecastDetailsLoadedMsg{SpotID: "honolua"})
	updated = updatedModel.(Model)
	if updated.current != homeScreen {
		t.Fatal("dashboard did not open after forecast and detail prefetch completed")
	}
}

func TestCompleteForecastCacheShowsInitialRefreshProgress(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	tracker := appTestCacheTracker{
		entries: map[string]surf.ForecastCacheEntry{
			"honolua": {
				SpotID:            "honolua",
				Forecast:          surf.Forecast{SpotID: "honolua", Slots: []surf.ForecastSlot{{Rating: "Fair"}}},
				ForecastUpdatedAt: updatedAt,
				Details:           surf.ForecastDetails{SpotID: "honolua"},
				DetailsUpdatedAt:  updatedAt,
			},
		},
	}
	model := New(
		resizeTestSearcher{},
		tracker,
		&appTestFullForecastProvider{},
		[]surf.Spot{{ID: "honolua"}},
		nil,
	)
	if model.current != loadingScreen || model.initialForecasts != 2 {
		t.Fatalf("cached initial state = screen %v loads %d, want loading screen with 2 refreshes", model.current, model.initialForecasts)
	}
	if loadingView := ansi.Strip(model.loading.View()); !strings.Contains(loadingView, "Locations loaded 0/1") {
		t.Fatalf("cached startup does not show initial fetch progress:\n%s", loadingView)
	}
	if model.Init() == nil {
		t.Fatal("cached startup did not retain its refresh commands")
	}

	updatedModel, _ := model.Update(dashboard.ForecastLoadedMsg{SpotID: "honolua"})
	updated := updatedModel.(Model)
	if updated.current != loadingScreen {
		t.Fatal("cached startup opened the dashboard before detail refresh completed")
	}
	if loadingView := ansi.Strip(updated.loading.View()); !strings.Contains(loadingView, "Fetching forecasts 0/1") ||
		strings.Contains(loadingView, "Locations loaded") {
		t.Fatalf("cached startup did not advance fetch progress:\n%s", loadingView)
	}

	updatedModel, _ = updated.Update(dashboard.ForecastDetailsLoadedMsg{
		SpotID: "honolua",
		Err:    errors.New("details unavailable"),
	})
	updated = updatedModel.(Model)
	if updated.current != homeScreen {
		t.Fatal("cached startup did not open the dashboard after all refreshes resolved")
	}
}

func TestEnterSkipsStartupWhenCacheIsAvailable(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	tracker := appTestCacheTracker{
		entries: map[string]surf.ForecastCacheEntry{
			"honolua": {
				SpotID:            "honolua",
				Forecast:          surf.Forecast{SpotID: "honolua", Slots: []surf.ForecastSlot{{Timestamp: updatedAt, Rating: "Fair"}}},
				ForecastUpdatedAt: updatedAt,
			},
		},
	}
	model := New(
		resizeTestSearcher{},
		tracker,
		&appTestFullForecastProvider{},
		[]surf.Spot{{ID: "honolua", Name: "Honolua Bay"}},
		nil,
	)
	if !model.loading.CanSkip() {
		t.Fatal("cached startup did not enable the skip control")
	}

	updatedModel, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := updatedModel.(Model)
	if cmd != nil || updated.current != homeScreen {
		t.Fatal("enter did not skip startup loading")
	}
	if pending := updated.dashboard.PendingInitialFetches(); pending != 2 {
		t.Fatalf("background refreshes after manual skip = %d, want 2", pending)
	}
}

func TestEnterDoesNotSkipStartupWithoutCache(t *testing.T) {
	t.Parallel()

	model := New(
		resizeTestSearcher{},
		resizeTestTracker{},
		&appTestFullForecastProvider{},
		[]surf.Spot{{ID: "honolua"}},
		nil,
	)
	if model.loading.CanSkip() {
		t.Fatal("uncached startup enabled the skip control")
	}

	updatedModel, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := updatedModel.(Model)
	if updated.current != loadingScreen {
		t.Fatal("enter skipped startup without cached data")
	}
}

func TestStartupWaitExpiryOpensDashboardWhileRefreshContinues(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	tracker := appTestCacheTracker{
		entries: map[string]surf.ForecastCacheEntry{
			"honolua": {
				SpotID:            "honolua",
				Forecast:          surf.Forecast{SpotID: "honolua", Slots: []surf.ForecastSlot{{Timestamp: updatedAt, Rating: "Fair"}}},
				ForecastUpdatedAt: updatedAt,
				Details:           surf.ForecastDetails{SpotID: "honolua"},
				DetailsUpdatedAt:  updatedAt,
			},
		},
	}
	model := New(
		resizeTestSearcher{},
		tracker,
		&appTestFullForecastProvider{},
		[]surf.Spot{{ID: "honolua", Name: "Honolua Bay"}},
		nil,
	)

	updatedModel, cmd := model.Update(startupWaitExpiredMsg{})
	updated := updatedModel.(Model)
	if cmd != nil || updated.current != homeScreen {
		t.Fatal("startup wait expiry did not open the cached dashboard")
	}
	if pending := updated.dashboard.PendingInitialFetches(); pending != 2 {
		t.Fatalf("background refreshes after startup expiry = %d, want 2", pending)
	}

	updatedModel, _ = updated.Update(dashboard.ForecastLoadedMsg{
		SpotID: "honolua",
		Err:    errors.New("base forecast still unavailable"),
	})
	updated = updatedModel.(Model)
	if updated.current != homeScreen || updated.dashboard.PendingInitialFetches() != 1 {
		t.Fatal("base forecast result did not continue updating after the dashboard opened")
	}

	updatedModel, _ = updated.Update(dashboard.ForecastDetailsLoadedMsg{
		SpotID: "honolua",
		Err:    errors.New("details still unavailable"),
	})
	updated = updatedModel.(Model)
	if updated.current != homeScreen || updated.dashboard.PendingInitialFetches() != 0 {
		t.Fatal("detail result did not finish in the background")
	}
}

func TestStartupWaitCommandOnlyRunsDuringLoading(t *testing.T) {
	t.Parallel()

	loadingModel := New(
		resizeTestSearcher{},
		resizeTestTracker{},
		&appTestForecastProvider{},
		[]surf.Spot{{ID: "honolua"}},
		nil,
	)
	if loadingModel.startupWaitCmd() == nil {
		t.Fatal("loading startup has no wait-limit command")
	}

	homeModel := New(resizeTestSearcher{}, resizeTestTracker{}, &appTestForecastProvider{}, nil, nil)
	if homeModel.startupWaitCmd() != nil {
		t.Fatal("empty dashboard unexpectedly starts a wait-limit command")
	}
}

func TestNoInitialForecastsStartsOnDashboard(t *testing.T) {
	t.Parallel()

	model := New(resizeTestSearcher{}, resizeTestTracker{}, &appTestForecastProvider{}, nil, nil)
	if model.current != homeScreen {
		t.Fatalf("initial screen = %v, want home", model.current)
	}
}

func TestDashboardKeysReachDashboardModel(t *testing.T) {
	t.Parallel()

	model := New(
		resizeTestSearcher{},
		resizeTestTracker{},
		nil,
		[]surf.Spot{{ID: "honolua", Name: "Honolua Bay"}},
		nil,
	)
	initialView := model.dashboard.View()

	updatedModel, _ := model.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	updated := updatedModel.(Model)
	selectedView := updated.dashboard.View()
	if selectedView == initialView {
		t.Fatal("j did not change dashboard selection rendering")
	}

	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	updated = updatedModel.(Model)
	if updated.dashboard.View() != initialView {
		t.Fatal("Esc did not restore the dashboard's initial unselected view")
	}
}

func TestDashboardSearchShortcutRequiresNoSelection(t *testing.T) {
	t.Parallel()

	model := New(
		resizeTestSearcher{},
		resizeTestTracker{},
		nil,
		[]surf.Spot{{ID: "honolua", Name: "Honolua Bay"}},
		nil,
	)
	updatedModel, _ := model.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	updated := updatedModel.(Model)
	if !updated.dashboard.HasSelection() {
		t.Fatal("j did not select a dashboard location")
	}

	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	updated = updatedModel.(Model)
	if updated.current != homeScreen || !updated.dashboard.HasSelection() {
		t.Fatal("/ opened search or cleared selection while a location was selected")
	}

	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	updated = updatedModel.(Model)
	if updated.dashboard.HasSelection() {
		t.Fatal("Esc did not clear dashboard selection")
	}
	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	updated = updatedModel.(Model)
	if updated.current != homeScreen {
		t.Fatal("s still opened search after its shortcut was removed")
	}
	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	updated = updatedModel.(Model)
	if updated.current != searchScreen {
		t.Fatal("/ did not open search after the dashboard selection was cleared")
	}
}

func TestRemovalConfirmationBlocksDashboardNavigation(t *testing.T) {
	t.Parallel()

	model := New(
		resizeTestSearcher{},
		resizeTestTracker{},
		nil,
		[]surf.Spot{{ID: "honolua", Name: "Honolua Bay"}},
		nil,
	)
	updatedModel, _ := model.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	updated := updatedModel.(Model)
	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	updated = updatedModel.(Model)
	if !updated.dashboard.ConfirmingRemoval() {
		t.Fatal("x did not open removal confirmation")
	}

	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	updated = updatedModel.(Model)
	if updated.current != homeScreen || !updated.dashboard.ConfirmingRemoval() {
		t.Fatal("search shortcut escaped the removal confirmation")
	}
	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	updated = updatedModel.(Model)
	if updated.current != homeScreen || !updated.dashboard.ConfirmingRemoval() {
		t.Fatal("quit shortcut escaped the removal confirmation")
	}
}

func TestForecastDetailsPopoverBlocksDashboardShortcuts(t *testing.T) {
	t.Parallel()

	model := New(
		resizeTestSearcher{},
		resizeTestTracker{},
		nil,
		[]surf.Spot{{ID: "honolua", Name: "Honolua Bay"}},
		nil,
	)
	updatedModel, _ := model.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	updated := updatedModel.(Model)
	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated = updatedModel.(Model)
	if !updated.dashboard.ShowingDetails() {
		t.Fatal("enter did not open dashboard details")
	}

	updatedModel, cmd := updated.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	updated = updatedModel.(Model)
	if cmd != nil || updated.current != homeScreen || !updated.dashboard.ShowingDetails() {
		t.Fatal("quit shortcut escaped the forecast details popover")
	}
	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	updated = updatedModel.(Model)
	if updated.current != homeScreen || !updated.dashboard.ShowingDetails() {
		t.Fatal("search shortcut escaped the forecast details popover")
	}

	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	updated = updatedModel.(Model)
	if updated.dashboard.ShowingDetails() || !updated.dashboard.HasSelection() {
		t.Fatal("escape did not close forecast details while preserving selection")
	}
}

type appTestForecastProvider struct{}

func (*appTestForecastProvider) Forecast(context.Context, string) (surf.Forecast, error) {
	return surf.Forecast{}, nil
}

type appTestFullForecastProvider struct{}

func (*appTestFullForecastProvider) Forecast(context.Context, string) (surf.Forecast, error) {
	return surf.Forecast{}, nil
}

func (*appTestFullForecastProvider) ForecastDetails(context.Context, string) (surf.ForecastDetails, error) {
	return surf.ForecastDetails{}, nil
}
