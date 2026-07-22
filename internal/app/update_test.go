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

func (resizeTestTracker) Remove(string) (bool, error) {
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

func TestDashboardSearchShortcutsRequireNoSelection(t *testing.T) {
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

	for _, shortcut := range []rune{'s', '/'} {
		updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: shortcut, Text: string(shortcut)})
		updated = updatedModel.(Model)
		if updated.current != homeScreen || !updated.dashboard.HasSelection() {
			t.Fatalf("%q opened search or cleared selection while a location was selected", shortcut)
		}
	}

	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	updated = updatedModel.(Model)
	if updated.dashboard.HasSelection() {
		t.Fatal("Esc did not clear dashboard selection")
	}
	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
	updated = updatedModel.(Model)
	if updated.current != searchScreen {
		t.Fatal("s did not open search after the dashboard selection was cleared")
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

	updatedModel, _ = updated.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
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

type appTestForecastProvider struct{}

func (*appTestForecastProvider) Forecast(context.Context, string) (surf.Forecast, error) {
	return surf.Forecast{}, nil
}
