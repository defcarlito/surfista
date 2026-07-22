package dashboard

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"surfista/internal/surf"
	"surfista/internal/ui"
)

type fakeSortStore struct {
	loaded string
	saved  []string
}

func (s *fakeSortStore) Remove(string) (bool, error) {
	return true, nil
}

func (s *fakeSortStore) LoadSortMode() (string, error) {
	return s.loaded, nil
}

func (s *fakeSortStore) SaveSortMode(mode string) error {
	s.saved = append(s.saved, mode)
	return nil
}

func TestSCyclesDashboardSortAndPreservesSelection(t *testing.T) {
	t.Parallel()

	spots := []surf.Spot{
		{ID: "first", Name: "First"},
		{ID: "second", Name: "Second"},
		{ID: "third", Name: "Third"},
	}
	store := &fakeSortStore{}
	model := New(nil, store, spots, nil)
	model.now = func() time.Time { return time.Date(2026, time.July, 21, 12, 30, 0, 0, time.UTC) }
	model.forecasts["first"] = forecastState{forecast: sortTestForecast("first", "Poor")}
	model.forecasts["second"] = forecastState{forecast: sortTestForecast("second", "Epic")}
	model.forecasts["third"] = forecastState{forecast: sortTestForecast("third", "Fair")}
	model, _ = model.Update(dashboardKey('j'))

	model, cmd := model.Update(dashboardKey('s'))
	if model.sortMode != SortConditionHighToLow {
		t.Fatalf("sort mode = %q, want condition high to low", model.sortMode)
	}
	if got, want := spotIDs(model.spots), []string{"second", "third", "first"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("condition order = %v, want %v", got, want)
	}
	if !model.HasSelection() || model.spots[model.selectedIndex].ID != "first" {
		t.Fatalf("selected spot was not preserved after sorting: index=%d spots=%v", model.selectedIndex, spotIDs(model.spots))
	}
	if cmd == nil {
		t.Fatal("cycling sort returned no persistence command")
	}
	message, ok := cmd().(SortModeSavedMsg)
	if !ok || message.Err != nil || message.Mode != SortConditionHighToLow {
		t.Fatalf("sort persistence message = %+v", message)
	}
	if !reflect.DeepEqual(store.saved, []string{string(SortConditionHighToLow)}) {
		t.Fatalf("saved sort modes = %v", store.saved)
	}
	view := model.View()
	if !strings.Contains(view, ui.DashboardSortStyle.Render("sorting by: conditions")) ||
		!strings.Contains(view, ui.DashboardHelpStyle.Render(" • s cycle sort")) {
		t.Fatalf("condition sort indicator is missing:\n%s", view)
	}

	model, _ = model.Update(dashboardKey('s'))
	if model.sortMode != SortTimeAdded {
		t.Fatalf("sort mode = %q, want time added", model.sortMode)
	}
	if got, want := spotIDs(model.spots), []string{"first", "second", "third"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("time-added order = %v, want %v", got, want)
	}
}

func TestPersistedConditionSortReordersAsForecastsArrive(t *testing.T) {
	t.Parallel()

	store := &fakeSortStore{loaded: string(SortConditionHighToLow)}
	model := New(nil, store, []surf.Spot{
		{ID: "first", Name: "First"},
		{ID: "second", Name: "Second"},
	}, nil)
	model.now = func() time.Time { return time.Date(2026, time.July, 21, 12, 30, 0, 0, time.UTC) }
	if model.sortMode != SortConditionHighToLow {
		t.Fatalf("loaded sort mode = %q, want condition high to low", model.sortMode)
	}

	model, _ = model.Update(ForecastLoadedMsg{SpotID: "second", Forecast: sortTestForecast("second", "Good")})
	if got, want := spotIDs(model.spots), []string{"second", "first"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order after forecast arrival = %v, want %v", got, want)
	}
}

func sortTestForecast(spotID, rating string) surf.Forecast {
	return surf.Forecast{
		SpotID: spotID,
		Slots: []surf.ForecastSlot{{
			Timestamp: time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC),
			Rating:    rating,
		}},
	}
}

func spotIDs(spots []surf.Spot) []string {
	ids := make([]string, len(spots))
	for index, spot := range spots {
		ids[index] = spot.ID
	}
	return ids
}
