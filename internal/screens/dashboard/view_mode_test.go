package dashboard

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"surfista/internal/surf"
	"surfista/internal/ui"
)

func TestVCyclesDashboardViews(t *testing.T) {
	t.Parallel()

	model := New(nil, nil, nil, nil)
	if got := model.viewMode; got != dashboardViewSurf {
		t.Fatalf("initial dashboard view = %v, want surf", got)
	}
	status := model.sortStatus(80)
	if plain := ansi.Strip(status); !strings.Contains(plain, "viewing: surf") {
		t.Fatalf("initial view status = %q, want surf label", plain)
	}
	if plain := ansi.Strip(status); !strings.Contains(plain, "s sorting by:") ||
		!strings.Contains(plain, "v viewing:") {
		t.Fatalf("dashboard status does not show inline key controls: %q", plain)
	}
	if !strings.Contains(status, ui.DashboardSortStyle.Render("viewing: surf")) {
		t.Fatal("view status does not use the white sorting-status style")
	}
	for _, key := range []string{"s", "v"} {
		if !strings.Contains(status, ui.DashboardHelpStyle.Render(key)) {
			t.Fatalf("inline %s control does not use the bottom-control style", key)
		}
	}
	statusLines := strings.Split(ansi.Strip(status), "\n")
	if len(statusLines) != 2 || strings.Index(statusLines[0], "sorting by:") != strings.Index(statusLines[1], "viewing:") {
		t.Fatalf("view status is not left-aligned beneath sorting: %q", statusLines)
	}
	if footer := ansi.Strip(model.dashboardFooter(80)); !strings.Contains(footer, "v view") {
		t.Fatalf("dashboard controls do not contain the view shortcut: %q", footer)
	}

	for _, want := range []dashboardViewMode{dashboardViewWind, dashboardViewSwell, dashboardViewSurf} {
		var cmd tea.Cmd
		model, cmd = model.Update(dashboardKey('v'))
		if cmd != nil {
			t.Fatalf("cycling to %s without a details provider returned a command", want.label())
		}
		if model.viewMode != want {
			t.Fatalf("dashboard view = %s, want %s", model.viewMode.label(), want.label())
		}
		if status := ansi.Strip(model.sortStatus(80)); !strings.Contains(status, "viewing: "+want.label()) {
			t.Fatalf("%s view status = %q", want.label(), status)
		}
	}
}

func TestDashboardViewsRenderSurfWindAndPrimarySwell(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	spot := surf.Spot{ID: "honolua", Name: "Honolua Bay"}
	slot := surf.ForecastSlot{
		Timestamp:  now,
		Rating:     "Fair",
		SurfHeight: surf.SurfHeight{Min: 3, Max: 4},
		Swells: []surf.Swell{{
			Height:    4,
			Period:    15,
			Direction: 202.5,
		}},
	}
	model := New(nil, nil, []surf.Spot{spot}, nil)
	model.now = func() time.Time { return now }
	model.forecasts[spot.ID] = forecastState{
		forecast:  surf.Forecast{SpotID: spot.ID, Slots: []surf.ForecastSlot{slot}},
		updatedAt: now,
	}
	model.details[spot.ID] = forecastDetailsState{
		details: surf.ForecastDetails{
			SpotID: spot.ID,
			Units:  surf.ForecastUnits{WindSpeed: "KTS"},
			Slots: []surf.ForecastDetailSlot{{
				Timestamp: slot.Timestamp,
				Wind: surf.Wind{
					Speed:         9.6,
					DirectionType: "OFFSHORE",
				},
			}},
		},
		updatedAt: now,
	}

	surfCard := ansi.Strip(model.spotCard(spot, 10, false))
	if !strings.Contains(surfCard, "3–4′") {
		t.Fatalf("surf view does not show surf height:\n%s", surfCard)
	}

	model, _ = model.Update(dashboardKey('v'))
	windCard := ansi.Strip(model.spotCard(spot, 10, false))
	if !strings.Contains(windCard, "off 10") || strings.Contains(windCard, "3–4′") {
		t.Fatalf("wind view did not replace surf height with wind:\n%s", windCard)
	}
	windStatus := model.sortStatus(80)
	if status := ansi.Strip(windStatus); !strings.Contains(status, "viewing: wind (kt)") {
		t.Fatalf("wind view status does not show its shared unit: %q", status)
	}
	if !strings.Contains(windStatus, ui.DashboardSubtitleStyle.Render("(kt)")) {
		t.Fatal("wind unit does not use the muted sorting-context style")
	}

	model, _ = model.Update(dashboardKey('v'))
	swellCard := ansi.Strip(model.spotCard(spot, 10, false))
	if !strings.Contains(swellCard, "4′ 15s SSW") || strings.Contains(swellCard, "3–4′") {
		t.Fatalf("swell view did not replace surf height with primary swell:\n%s", swellCard)
	}
}

func TestDashboardMetricFormattingAdaptsToNarrowSlots(t *testing.T) {
	t.Parallel()

	wind := surf.Wind{Speed: 6, DirectionType: "CROSS-SHORE"}
	if got := compactDashboardWind(wind, 10); got != "cross 6" {
		t.Fatalf("wide compact wind = %q, want %q", got, "cross 6")
	}
	if got := compactDashboardWind(wind, 6); got != "x 6" {
		t.Fatalf("narrow compact wind = %q, want %q", got, "x 6")
	}

	swell := surf.Swell{Height: 4, Period: 15, Direction: 202.5}
	if got := compactDashboardSwell(swell, 10); got != "4′ 15s SSW" {
		t.Fatalf("wide compact swell = %q, want %q", got, "4′ 15s SSW")
	}
	if got := compactDashboardSwell(swell, 7); got != "15s SSW" {
		t.Fatalf("narrow compact swell = %q, want %q", got, "15s SSW")
	}
}

func TestWindViewFetchesOnlyMissingDetailsTwoAtATime(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	spots := []surf.Spot{
		{ID: "cached"},
		{ID: "second"},
		{ID: "third"},
		{ID: "fourth"},
		{ID: "fifth"},
	}
	cache := &fakeForecastCache{entries: make(map[string]surf.ForecastCacheEntry, len(spots))}
	for _, spot := range spots {
		cache.entries[spot.ID] = surf.ForecastCacheEntry{
			SpotID: spot.ID,
			Forecast: surf.Forecast{
				SpotID: spot.ID,
				Slots:  []surf.ForecastSlot{{Timestamp: now, Rating: "Fair"}},
			},
			ForecastUpdatedAt: now,
		}
	}
	cached := cache.entries["cached"]
	cached.Details = surf.ForecastDetails{
		SpotID: "cached",
		Slots:  []surf.ForecastDetailSlot{{Timestamp: now}},
	}
	cached.DetailsUpdatedAt = now
	cache.entries["cached"] = cached

	provider := &fakeForecastProvider{details: surf.ForecastDetails{
		Slots: []surf.ForecastDetailSlot{{Timestamp: now}},
	}}
	model := New(provider, cache, spots, nil)
	model, cmd := model.Update(dashboardKey('v'))
	if cmd == nil {
		t.Fatal("activating wind view did not start missing detail fetches")
	}
	if active, queued := detailWorkCounts(model); active != 2 || queued != 2 {
		t.Fatalf("wind detail work = %d active, %d queued; want 2 active, 2 queued", active, queued)
	}

	messages := commandMessages(t, cmd)
	results := make([]ForecastDetailsLoadedMsg, 0, 2)
	for _, message := range messages {
		if result, ok := message.(ForecastDetailsLoadedMsg); ok {
			results = append(results, result)
		}
	}
	if len(results) != 2 {
		t.Fatalf("initial wind fetch results = %d, want 2", len(results))
	}
	if !reflect.DeepEqual(provider.detailSpotIDs, []string{"second", "third"}) {
		t.Fatalf("initial detail provider calls = %v, want only first two missing spots", provider.detailSpotIDs)
	}

	model, next := model.Update(results[0])
	if next == nil {
		t.Fatal("finishing one wind fetch did not start the next queued location")
	}
	if active, queued := detailWorkCounts(model); active != 2 || queued != 1 {
		t.Fatalf("continued wind detail work = %d active, %d queued; want 2 active, 1 queued", active, queued)
	}
	nextMessages := commandMessages(t, next)
	if len(nextMessages) != 1 {
		t.Fatalf("next wind queue step produced %d messages, want 1", len(nextMessages))
	}
	nextResult, ok := nextMessages[0].(ForecastDetailsLoadedMsg)
	if !ok || nextResult.SpotID != "fourth" {
		t.Fatalf("next wind queue message = %#v, want fourth location", nextMessages[0])
	}
}

func detailWorkCounts(model Model) (active, queued int) {
	for _, state := range model.details {
		if state.loading {
			active++
		}
		if state.queued {
			queued++
		}
	}
	return active, queued
}
