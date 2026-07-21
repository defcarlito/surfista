package app

import (
	"context"
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

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

type appTestForecastProvider struct{}

func (*appTestForecastProvider) Forecast(context.Context, string) (surf.Forecast, error) {
	return surf.Forecast{}, nil
}
