package dashboard

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

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
		!strings.Contains(ansi.Strip(view), "s cycle sort") {
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

func TestConditionSortUsesCurrentWaveHeightAsTieBreaker(t *testing.T) {
	t.Parallel()

	model := New(nil, nil, []surf.Spot{
		{ID: "lower", Name: "Lower"},
		{ID: "higher", Name: "Higher"},
		{ID: "same-max", Name: "Same Max"},
		{ID: "better-condition", Name: "Better Condition"},
		{ID: "same-range", Name: "Same Range"},
	}, nil)
	model.now = func() time.Time { return time.Date(2026, time.July, 21, 12, 30, 0, 0, time.UTC) }
	model.forecasts["lower"] = forecastState{forecast: surf.Forecast{
		SpotID: "lower",
		Slots: []surf.ForecastSlot{
			{
				Timestamp:  time.Date(2026, time.July, 21, 9, 0, 0, 0, time.UTC),
				Rating:     "Fair",
				SurfHeight: surf.SurfHeight{Min: 10, Max: 12},
			},
			{
				Timestamp:  time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC),
				Rating:     "Fair",
				SurfHeight: surf.SurfHeight{Min: 1, Max: 3},
			},
		},
	}}
	model.forecasts["higher"] = forecastState{forecast: sortTestForecastWithHeight("higher", "Fair", 2, 5)}
	model.forecasts["same-max"] = forecastState{forecast: sortTestForecastWithHeight("same-max", "Fair", 1, 5)}
	model.forecasts["better-condition"] = forecastState{forecast: sortTestForecastWithHeight("better-condition", "Good", 1, 2)}
	model.forecasts["same-range"] = forecastState{forecast: sortTestForecastWithHeight("same-range", "Fair", 2, 5)}
	model.sortMode = SortConditionHighToLow

	model.applySort()

	want := []string{"better-condition", "higher", "same-range", "same-max", "lower"}
	if got := spotIDs(model.spots); !reflect.DeepEqual(got, want) {
		t.Fatalf("condition and wave-height order = %v, want %v", got, want)
	}
}

func TestConditionSortStatusAlwaysSaysBestRightNowForTodayAtNight(t *testing.T) {
	t.Parallel()

	model := New(nil, nil, nil, nil)
	model.now = func() time.Time { return time.Date(2026, time.July, 21, 2, 30, 0, 0, time.UTC) }
	model.sortMode = SortConditionHighToLow

	status := model.sortStatus(100)
	if !strings.Contains(status, ui.DashboardSortStyle.Render("sorting by: conditions")) {
		t.Fatalf("today status is missing condition label: %q", ansi.Strip(status))
	}
	if !strings.Contains(status, ui.DashboardSubtitleStyle.Render("best right now")) {
		t.Fatalf("nighttime today status is missing best-right-now context: %q", ansi.Strip(status))
	}
	if strings.Contains(status, "best overall") {
		t.Fatalf("today status uses future-day wording: %q", ansi.Strip(status))
	}
}

func TestFutureConditionSortUsesDaylightAverageAndBestThreeHourWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC)
	forecastDate := time.Date(2026, time.July, 22, 0, 0, 0, 0, time.UTC)
	spots := []surf.Spot{
		{ID: "night-epic", Name: "Night Epic"},
		{ID: "short-peak", Name: "Short Peak"},
		{ID: "consistent", Name: "Consistent"},
		{ID: "missing-sunlight", Name: "Missing Sunlight"},
	}
	model := New(nil, nil, spots, nil)
	model.now = func() time.Time { return now }
	model.forecasts["night-epic"] = forecastState{forecast: futureSortForecast("night-epic", forecastDate, func(hour int) string {
		if hour < 7 {
			return "Epic"
		}
		return "Poor"
	})}
	model.forecasts["short-peak"] = forecastState{forecast: futureSortForecast("short-peak", forecastDate, func(hour int) string {
		if hour >= 7 && hour <= 9 {
			return "Epic"
		}
		return "Poor"
	})}
	model.forecasts["consistent"] = forecastState{forecast: futureSortForecast("consistent", forecastDate, func(int) string {
		return "Good"
	})}
	model.forecasts["missing-sunlight"] = forecastState{forecast: futureSortForecast("missing-sunlight", forecastDate, func(int) string {
		return "Epic"
	})}

	daylight := surf.SunlightDay{
		Sunrise: time.Date(2026, time.July, 22, 6, 30, 0, 0, time.UTC),
		Sunset:  time.Date(2026, time.July, 22, 18, 30, 0, 0, time.UTC),
	}
	for _, spotID := range []string{"night-epic", "short-peak", "consistent"} {
		model.details[spotID] = forecastDetailsState{details: surf.ForecastDetails{
			SpotID:   spotID,
			Sunlight: []surf.SunlightDay{daylight},
		}}
	}

	peakValue := futureConditionSortValue(
		model.forecasts["short-peak"].forecast,
		model.details["short-peak"].details,
		now,
		1,
	)
	if !peakValue.available || peakValue.score != 3.625 {
		t.Fatalf("short-peak score = %+v, want 75%% daylight average + 25%% best window = 3.625", peakValue)
	}

	model.sortMode = SortConditionHighToLow
	model.moveForecastDay(1)
	if model.forecastDayOffset != 1 {
		t.Fatalf("forecast day offset = %d, want 1", model.forecastDayOffset)
	}
	want := []string{"consistent", "short-peak", "night-epic", "missing-sunlight"}
	if got := spotIDs(model.spots); !reflect.DeepEqual(got, want) {
		t.Fatalf("future daylight-score order = %v, want %v", got, want)
	}

	status := model.sortStatus(100)
	if !strings.Contains(status, ui.DashboardSortStyle.Render("sorting by: conditions")) ||
		!strings.Contains(status, ui.DashboardSubtitleStyle.Render("best overall for 7/22")) {
		t.Fatalf("future sort status is missing contextual label: %q", ansi.Strip(status))
	}
	if strings.Contains(status, "best right now") {
		t.Fatalf("future sort status still says best right now: %q", ansi.Strip(status))
	}
}

func TestFutureConditionSortReordersWhenSunlightArrives(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC)
	forecastDate := time.Date(2026, time.July, 22, 0, 0, 0, 0, time.UTC)
	model := New(nil, nil, []surf.Spot{
		{ID: "first", Name: "First"},
		{ID: "second", Name: "Second"},
	}, nil)
	model.now = func() time.Time { return now }
	model.forecasts["first"] = forecastState{forecast: futureSortForecast("first", forecastDate, func(int) string { return "Poor" })}
	model.forecasts["second"] = forecastState{forecast: futureSortForecast("second", forecastDate, func(int) string { return "Good" })}
	model.forecastDayOffset = 1
	model.sortMode = SortConditionHighToLow
	model.applySort()

	daylight := surf.SunlightDay{
		Sunrise: time.Date(2026, time.July, 22, 6, 30, 0, 0, time.UTC),
		Sunset:  time.Date(2026, time.July, 22, 18, 30, 0, 0, time.UTC),
	}
	model, _ = model.Update(ForecastDetailsLoadedMsg{
		SpotID: "second",
		Details: surf.ForecastDetails{
			SpotID:   "second",
			Sunlight: []surf.SunlightDay{daylight},
		},
	})
	if got, want := spotIDs(model.spots), []string{"second", "first"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("order after future daylight arrival = %v, want %v", got, want)
	}
}

func sortTestForecast(spotID, rating string) surf.Forecast {
	return sortTestForecastWithHeight(spotID, rating, 0, 0)
}

func sortTestForecastWithHeight(spotID, rating string, minHeight, maxHeight float64) surf.Forecast {
	return surf.Forecast{
		SpotID: spotID,
		Slots: []surf.ForecastSlot{{
			Timestamp: time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC),
			Rating:    rating,
			SurfHeight: surf.SurfHeight{
				Min: minHeight,
				Max: maxHeight,
			},
		}},
	}
}

func futureSortForecast(spotID string, date time.Time, ratingForHour func(int) string) surf.Forecast {
	forecast := surf.Forecast{SpotID: spotID}
	for hour := 0; hour < 24; hour++ {
		forecast.Slots = append(forecast.Slots, surf.ForecastSlot{
			Timestamp: time.Date(date.Year(), date.Month(), date.Day(), hour, 0, 0, 0, time.UTC),
			Rating:    ratingForHour(hour),
			SurfHeight: surf.SurfHeight{
				Min: 2,
				Max: 4,
			},
		})
	}
	return forecast
}

func spotIDs(spots []surf.Spot) []string {
	ids := make([]string, len(spots))
	for index, spot := range spots {
		ids[index] = spot.ID
	}
	return ids
}
