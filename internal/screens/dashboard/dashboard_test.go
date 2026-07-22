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
	model := New(provider, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
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
	if message := cmd(); message.(ForecastLoadedMsg).SpotID != "pine-trees" {
		t.Fatalf("message = %+v", message)
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

	model := New(nil, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	plain := ansi.Strip(model.View())
	for _, want := range []string{"↑/k ↓/j navigate", "/ search Surfline", "q quit"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("unselected dashboard help does not contain %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "s or /") {
		t.Fatalf("unselected dashboard help still offers s as a search shortcut:\n%s", plain)
	}
	for _, unavailable := range []string{"x remove", "esc unselect"} {
		if strings.Contains(plain, unavailable) {
			t.Fatalf("unselected dashboard help contains unavailable action %q:\n%s", unavailable, plain)
		}
	}

	model, _ = model.Update(dashboardKey('j'))
	plain = ansi.Strip(model.View())
	for _, want := range []string{"↑/k ↓/j navigate", "x remove", "esc unselect", "q quit"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("selected dashboard help does not contain %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "search Surfline") {
		t.Fatalf("selected dashboard help still offers search:\n%s", plain)
	}

	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	plain = ansi.Strip(model.View())
	if !strings.Contains(plain, "search Surfline") || strings.Contains(plain, "x remove") || strings.Contains(plain, "esc unselect") {
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
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	plain := ansi.Strip(model.View())
	lines := strings.Split(plain, "\n")
	if len(lines) != 24 {
		t.Fatalf("view height = %d, want terminal height 24:\n%s", len(lines), plain)
	}
	if !strings.Contains(lines[0], "Today's surf conditions") {
		t.Fatalf("title is not pinned to the first line: %q", lines[0])
	}
	if !strings.Contains(lines[2], "12a") || !strings.Contains(lines[2], "3a") {
		t.Fatalf("time header is not pinned near the top: %q", lines[2])
	}
	if !strings.Contains(lines[3], "now") {
		t.Fatalf("current-time label is not directly below the time header: %q", lines[3])
	}
	if strings.TrimSpace(lines[4]) != "" {
		t.Fatalf("expected one blank row between the current-time label and top arrow slot: %q", lines[4])
	}
	if !strings.Contains(lines[len(lines)-2], "↑/k ↓/j navigate") || strings.TrimSpace(lines[len(lines)-1]) != "" {
		t.Fatalf("controls are not one row above the bottom: %q / %q", lines[len(lines)-2], lines[len(lines)-1])
	}
	if strings.TrimSpace(lines[len(lines)-3]) != "" {
		t.Fatalf("sort indicator and controls do not have a blank row between them: %q", lines[len(lines)-3])
	}
	if !strings.Contains(lines[len(lines)-4], "sorting by: time added") || !strings.Contains(lines[len(lines)-4], "s cycle sort") {
		t.Fatalf("sort indicator is not above the controls: %q", lines[len(lines)-4])
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
	if arrowLine := standaloneIndicatorLine(lines, "↓"); arrowLine != len(lines)-5 {
		t.Fatalf("down arrow line = %d, want bottom of location viewport at %d:\n%s", arrowLine, len(lines)-5, plain)
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
		t.Fatalf("up arrow line = %d, want below the time-header gap at 5:\n%s", arrowLine, plain)
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
	if !strings.Contains(lines[0], "Today's surf conditions") || !strings.Contains(lines[2], "12a") ||
		!strings.Contains(lines[len(lines)-2], "↑/k ↓/j navigate") || strings.TrimSpace(lines[len(lines)-1]) != "" {
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
	if !strings.Contains(lines[0], "Today's surf conditions") || !strings.Contains(lines[1], "12a") {
		t.Fatalf("compact layout did not keep title and time at the top:\n%s", plain)
	}
	if !strings.Contains(lines[len(lines)-2], "↑/k ↓/j navigate") || strings.TrimSpace(lines[len(lines)-1]) != "" {
		t.Fatalf("compact layout did not keep controls at the bottom:\n%s", plain)
	}
	if !strings.Contains(lines[len(lines)-3], "sorting by: time added") {
		t.Fatalf("compact layout did not keep sort indicator above controls:\n%s", plain)
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
	for _, want := range []string{"Today's surf conditions", "Remove Honolua Bay from tracked locations?", "Enter remove", "Esc cancel"} {
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
	assertDialogTextCentered(t, model.removalDialog(), "Enter remove • Esc cancel")
	assertRemovalDialogCentered(t, model)

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

func assertRemovalDialogCentered(t *testing.T, model Model) {
	t.Helper()

	dialog := ansi.Strip(model.removalDialog())
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
	if !strings.Contains(plain, "Could not remove: disk unavailable") || !strings.Contains(plain, "Enter retry") {
		t.Fatalf("failed removal view does not show retry guidance:\n%s", plain)
	}
}
