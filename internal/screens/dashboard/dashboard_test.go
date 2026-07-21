package dashboard

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"surfista/internal/surf"
	"surfista/internal/ui"
)

type fakeForecastProvider struct {
	forecast surf.Forecast
	err      error
	spotIDs  []string
}

func (p *fakeForecastProvider) Forecast(_ context.Context, spotID string) (surf.Forecast, error) {
	p.spotIDs = append(p.spotIDs, spotID)
	return p.forecast, p.err
}

func TestInitFetchesFavoriteForecast(t *testing.T) {
	t.Parallel()

	provider := &fakeForecastProvider{forecast: surf.Forecast{SpotID: "honolua"}}
	model := New(provider, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	cmd := model.Init()
	if cmd == nil {
		t.Fatal("Init returned no forecast command")
	}

	message := cmd()
	loaded, ok := message.(ForecastLoadedMsg)
	if !ok {
		t.Fatalf("message = %T, want ForecastLoadedMsg", message)
	}
	if loaded.SpotID != "honolua" || loaded.Forecast.SpotID != "honolua" || loaded.Err != nil {
		t.Fatalf("message = %+v", loaded)
	}
	if len(provider.spotIDs) != 1 || provider.spotIDs[0] != "honolua" {
		t.Fatalf("provider calls = %v", provider.spotIDs)
	}
}

func TestForecastResultUpdatesOnlyTrackedSpot(t *testing.T) {
	t.Parallel()

	model := New(nil, []surf.Spot{{ID: "honolua"}}, nil)
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

	model := New(nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
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
	if strings.Contains(rendered, "\x1b[48") {
		t.Fatal("current forecast cell still uses a background highlight")
	}
	currentOutline := ui.DashboardCurrentBorderStyle.Render(strings.Repeat("─", 7))
	if !strings.Contains(rendered, currentOutline) {
		t.Fatal("current forecast cell does not use the highlighted outline")
	}
}

func TestHeightUsesFeetInsteadOfBodyRelation(t *testing.T) {
	t.Parallel()

	height := surf.SurfHeight{Min: 1, Max: 2, Plus: true, HumanRelation: "Knee to thigh"}
	if got := formatHeight(height); got != "1–2′+" {
		t.Fatalf("height = %q, want %q", got, "1–2′+")
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

	model := New(nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
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

	model := New(nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
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
	model := New(provider, nil, nil)
	cmd := model.Add(surf.Spot{ID: "pine-trees", Name: "Pine Trees"})
	if cmd == nil {
		t.Fatal("Add returned no forecast command")
	}
	if message := cmd(); message.(ForecastLoadedMsg).SpotID != "pine-trees" {
		t.Fatalf("message = %+v", message)
	}
	if len(model.spots) != 1 || !model.forecasts["pine-trees"].loading {
		t.Fatalf("dashboard state = %+v", model)
	}
}

func TestLocationSelectionStartsEmptyAndMovesWithJK(t *testing.T) {
	t.Parallel()

	model := New(nil, []surf.Spot{
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

func TestSelectedLocationUsesWhiteBordersThroughout(t *testing.T) {
	t.Parallel()

	spot := surf.Spot{ID: "honolua", Name: "Honolua Bay"}
	model := New(nil, []surf.Spot{spot}, nil)
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
	if got := segmentedBorder("╰", "┴", "╯", 7, -1, true); got != wantSegmentedBorder {
		t.Fatalf("selected forecast grid border was not entirely white: %q", got)
	}

	cells := []string{"rating", "height"}
	wantGridLine := ui.DashboardSelectedBorderStyle.Render("│") + cells[0] +
		ui.DashboardSelectedBorderStyle.Render("│") + cells[1] +
		ui.DashboardSelectedBorderStyle.Render("│")
	if got := segmentedLine(cells, -1, true); got != wantGridLine {
		t.Fatalf("selected forecast grid cells were not outlined in white: %q", got)
	}
}

func dashboardKey(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code, Text: string(code)}
}
