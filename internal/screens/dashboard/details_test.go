package dashboard

import (
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"surfista/internal/surf"
	"surfista/internal/ui"
)

func TestEnterOpensCurrentForecastDetailsAndEscapeCloses(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 21, 12, 30, 0, 0, time.UTC)
	temperature := 86.0
	provider := &fakeForecastProvider{details: surf.ForecastDetails{
		SpotID: "honolua",
		Units: surf.ForecastUnits{
			WindSpeed:   "KTS",
			TideHeight:  "FT",
			Temperature: "F",
		},
		Slots: []surf.ForecastDetailSlot{{
			Timestamp: time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC),
			Wind: surf.Wind{
				Speed:         3,
				Gust:          4,
				Direction:     216,
				DirectionType: "CROSS_SHORE",
			},
			Temperature: &temperature,
		}},
		Tides: []surf.TidePoint{
			{Timestamp: time.Date(2026, time.July, 21, 9, 0, 0, 0, time.UTC), Type: "LOW", Height: 1},
			{Timestamp: time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC), Type: "NORMAL", Height: 2.5},
			{Timestamp: time.Date(2026, time.July, 21, 13, 0, 0, 0, time.UTC), Type: "NORMAL", Height: 3.5},
			{Timestamp: time.Date(2026, time.July, 21, 15, 0, 0, 0, time.UTC), Type: "HIGH", Height: 5},
		},
	}}
	model := New(provider, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model.now = func() time.Time { return now }
	model.forecasts["honolua"] = forecastState{forecast: surf.Forecast{
		SpotID: "honolua",
		Slots: []surf.ForecastSlot{{
			Timestamp:  time.Date(2026, time.July, 21, 12, 0, 0, 0, time.UTC),
			Rating:     "Fair to Good",
			SurfHeight: surf.SurfHeight{Min: 3, Max: 4, Plus: true, HumanRelation: "Waist to shoulder"},
			Swells: []surf.Swell{
				{Height: 2.7, Period: 14, Direction: 216},
				{Height: 1.6, Period: 12, Direction: 187},
			},
		}},
	}}
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	for _, message := range commandMessages(t, model.Init()) {
		if _, ok := message.(ForecastDetailsLoadedMsg); ok {
			model, _ = model.Update(message)
		}
	}
	model, _ = model.Update(dashboardKey('j'))

	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !model.ShowingDetails() || cmd != nil {
		t.Fatalf("enter did not open prefetched details immediately: open=%v cmd=%v", model.ShowingDetails(), cmd)
	}
	if !reflect.DeepEqual(provider.detailSpotIDs, []string{"honolua"}) {
		t.Fatalf("detail provider calls = %v", provider.detailSpotIDs)
	}
	renderedDialog := model.detailsDialog()
	if !strings.Contains(renderedDialog, ui.DashboardCurrentHourStyle.Render("12p")) {
		t.Fatalf("current time does not use the dashboard time highlight:\n%s", ansi.Strip(renderedDialog))
	}
	if strings.Contains(renderedDialog, ui.DashboardCurrentHourStyle.Render("now")) ||
		strings.Contains(renderedDialog, ui.DashboardCurrentHourStyle.Render("3–4′+")) {
		t.Fatalf("current-time background extends beyond the time label:\n%s", ansi.Strip(renderedDialog))
	}

	plain := ansi.Strip(model.View())
	for _, want := range []string{
		"Honolua Bay",
		"fair to good",
		"Surf height",
		"3–4′+",
		"Waist to shoulder",
		"Swell",
		"2.7′ 14s SW",
		"Wind",
		"3 kts SW",
		"gusts 4 kts",
		"cross-shore",
		"Tide",
		"2.5′ rising",
		"l 9:00a 1′",
		"h 3:00p 5′",
		"Temperature",
		"86°F",
		"now 12p",
		"esc close",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("detail popover does not contain %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "current slot") || strings.Contains(plain, "CONDITION") {
		t.Fatalf("detail header contains removed labels:\n%s", plain)
	}
	assertDetailPanelsShareRow(t, plain)

	model, _ = model.Update(dashboardKey('j'))
	if model.selectedIndex != 0 {
		t.Fatalf("selection moved while details were open: %d", model.selectedIndex)
	}
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	if model.ShowingDetails() || model.selectedIndex != 0 {
		t.Fatalf("escape did not close details while preserving selection: open=%v selection=%d", model.ShowingDetails(), model.selectedIndex)
	}
}

func TestDetailsShowEveryHourAndMarkExactCurrentHour(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 21, 14, 30, 0, 0, time.UTC)
	forecast := surf.Forecast{SpotID: "honolua"}
	for hour := 0; hour <= 24; hour++ {
		forecast.Slots = append(forecast.Slots, surf.ForecastSlot{
			Timestamp: time.Date(2026, time.July, 21, hour, 0, 0, 0, time.UTC),
			Rating:    "Fair",
		})
	}

	model := New(nil, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model.now = func() time.Time { return now }
	model.forecasts["honolua"] = forecastState{forecast: forecast}
	model.details["honolua"] = forecastDetailsState{details: surf.ForecastDetails{SpotID: "honolua"}}
	model.detailsSpot = model.spots[0]

	rows := model.detailsForecastRows()
	wantLabels := []string{
		"12a", "1a", "2a", "3a", "4a", "5a", "6a", "7a", "8a", "9a", "10a", "11a",
		"12p", "1p", "2p", "3p", "4p", "5p", "6p", "7p", "8p", "9p", "10p", "11p", "12a",
	}
	if len(rows) != len(wantLabels) {
		t.Fatalf("detail rows = %d, want %d hourly rows", len(rows), len(wantLabels))
	}
	for index, want := range wantLabels {
		if rows[index].timeLabel != want {
			t.Fatalf("row %d label = %q, want %q", index, rows[index].timeLabel, want)
		}
		if rows[index].current != (want == "2p") {
			t.Fatalf("row %q current = %v, want %v", want, rows[index].current, want == "2p")
		}
	}
	if !isCurrentDashboardHour(12, now) || isCurrentDashboardHour(15, now) {
		t.Fatal("dashboard no longer groups 2pm into its 12pm three-hour slot")
	}
}

func TestDetailsCurrentHourUsesTerminalClockInsteadOfSpotOffset(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 22, 0, 43, 0, 0, time.UTC)
	offset := -1 * time.Hour
	forecast := surf.Forecast{SpotID: "honolua", UTCOffset: offset}
	for hour := 0; hour <= 24; hour++ {
		localWallTime := time.Date(2026, time.July, 21, hour, 0, 0, 0, time.UTC)
		forecast.Slots = append(forecast.Slots, surf.ForecastSlot{
			Timestamp: localWallTime.Add(-offset),
			Rating:    "Fair",
		})
	}

	model := New(nil, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model.now = func() time.Time { return now }
	model.forecasts["honolua"] = forecastState{forecast: forecast}
	model.details["honolua"] = forecastDetailsState{details: surf.ForecastDetails{SpotID: "honolua"}}
	model.detailsSpot = model.spots[0]

	rows := model.detailsForecastRows()
	if len(rows) == 0 {
		t.Fatal("detail rows are empty")
	}
	for _, row := range rows {
		if row.current && row.timeLabel != "12a" {
			t.Fatalf("current detail row = %q, want 12a", row.timeLabel)
		}
	}
	if !rows[0].current || rows[0].timeLabel != "12a" {
		t.Fatalf("first detail row = %+v, want current 12a row", rows[0])
	}
}

func TestDetailsRowsScrollWithJKWithoutChangingSelection(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 21, 12, 30, 0, 0, time.UTC)
	forecast := surf.Forecast{SpotID: "honolua"}
	for hour := 0; hour <= 24; hour++ {
		forecast.Slots = append(forecast.Slots, surf.ForecastSlot{
			Timestamp:  time.Date(2026, time.July, 21, hour, 0, 0, 0, time.UTC),
			Rating:     "Fair",
			SurfHeight: surf.SurfHeight{Min: 2, Max: 3},
		})
	}
	model := New(nil, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model.now = func() time.Time { return now }
	model.forecasts["honolua"] = forecastState{forecast: forecast}
	model.details["honolua"] = forecastDetailsState{details: surf.ForecastDetails{SpotID: "honolua"}}
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	model, _ = model.Update(dashboardKey('j'))
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

	if model.detailsScroll == 0 {
		t.Fatal("details did not open near the current time slot")
	}
	initialOffset := model.detailsScroll
	plain := ansi.Strip(model.detailsDialog())
	if !strings.Contains(plain, "now 12p") || !strings.Contains(plain, "↓/j scroll down") || !strings.Contains(plain, "↑/k scroll up") {
		t.Fatalf("initial detail viewport is missing current-row context or controls:\n%s", plain)
	}

	model, _ = model.Update(dashboardKey('j'))
	if model.detailsScroll != initialOffset+1 || model.selectedIndex != 0 {
		t.Fatalf("j changed state to offset=%d selection=%d, want offset=%d selection=0", model.detailsScroll, model.selectedIndex, initialOffset+1)
	}
	model, _ = model.Update(dashboardKey('k'))
	if model.detailsScroll != initialOffset || model.selectedIndex != 0 {
		t.Fatalf("k changed state to offset=%d selection=%d, want offset=%d selection=0", model.detailsScroll, model.selectedIndex, initialOffset)
	}
	for strings.Contains(ansi.Strip(model.detailsDialog()), "↓/j scroll down") {
		model, _ = model.Update(dashboardKey('j'))
	}
	bottom := ansi.Strip(model.detailsDialog())
	if strings.Contains(bottom, "↓/j scroll down") || !strings.Contains(bottom, "↑/k scroll up") {
		t.Fatalf("bottom viewport shows invalid controls:\n%s", bottom)
	}
	for model.detailsScroll > 0 {
		model, _ = model.Update(dashboardKey('k'))
	}
	top := ansi.Strip(model.detailsDialog())
	if strings.Contains(top, "↑/k scroll up") || !strings.Contains(top, "↓/j scroll down") {
		t.Fatalf("top viewport shows invalid controls:\n%s", top)
	}
}

func TestForecastDetailsArePrefetchedAndOpenedFromCache(t *testing.T) {
	t.Parallel()

	provider := &fakeForecastProvider{details: surf.ForecastDetails{SpotID: "honolua"}}
	model := New(provider, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	for _, message := range commandMessages(t, model.Init()) {
		if _, ok := message.(ForecastDetailsLoadedMsg); ok {
			model, _ = model.Update(message)
		}
	}
	model, _ = model.Update(dashboardKey('j'))
	model, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil || !model.ShowingDetails() {
		t.Fatalf("prefetched open = open %v command %v, want open without command", model.ShowingDetails(), cmd)
	}
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEsc})

	model, cmd = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil || !model.ShowingDetails() {
		t.Fatalf("cached open = open %v command %v, want open without command", model.ShowingDetails(), cmd)
	}
	if !reflect.DeepEqual(provider.detailSpotIDs, []string{"honolua"}) {
		t.Fatalf("prefetched details refetched: %v", provider.detailSpotIDs)
	}
}

func TestDetailsDialogKeepsPanelsInOneRowAtNarrowWidth(t *testing.T) {
	t.Parallel()

	model := New(nil, nil, []surf.Spot{{ID: "honolua", Name: "Honolua Bay"}}, nil)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	model, _ = model.Update(dashboardKey('j'))
	model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	dialog := model.detailsDialog()
	if got := lipgloss.Width(dialog); got > 78 {
		t.Fatalf("narrow detail dialog width = %d, want at most 78:\n%s", got, ansi.Strip(dialog))
	}

	plain := ansi.Strip(dialog)
	titles := []string{"Surf", "Swell", "Wind", "Tide", "Temp"}
	for _, line := range strings.Split(plain, "\n") {
		lastIndex := -1
		for _, title := range titles {
			index := strings.Index(line, title)
			if index < 0 || index <= lastIndex {
				lastIndex = -1
				break
			}
			lastIndex = index
		}
		if lastIndex >= 0 {
			return
		}
	}
	t.Fatalf("compact detail panels are not in one row:\n%s", plain)
}

func TestDetailValuesRoundToTenthsAndDescriptionsWrap(t *testing.T) {
	t.Parallel()

	if got := formatDetailNumber(1.3944324); got != "1.4" {
		t.Fatalf("rounded decimal = %q, want 1.4", got)
	}
	if got := formatDetailNumber(3); got != "3" {
		t.Fatalf("whole number = %q, want 3", got)
	}
	if got := formatDetailHeight(surf.SurfHeight{Min: 1.3944324, Max: 2.86}); got != "1.4–2.9′" {
		t.Fatalf("rounded surf height = %q, want 1.4–2.9′", got)
	}
	if got, want := wrapDetailText("Chest to head", 8), []string{"Chest to", "head"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("wrapped description = %v, want %v", got, want)
	}
	if got := ansi.Strip(padDetailCell("3", 5)); got != "  3  " {
		t.Fatalf("centered detail cell = %q, want %q", got, "  3  ")
	}
}

func TestCurrentTimeBackgroundMatchesForecastRowHeight(t *testing.T) {
	t.Parallel()

	lines := detailTimeLabelLines(forecastDetailRow{timeLabel: "12p", current: true})
	if len(lines) != detailsForecastRowHeight {
		t.Fatalf("highlighted time height = %d, want %d", len(lines), detailsForecastRowHeight)
	}
	middle := detailsForecastRowHeight / 2
	for index, line := range lines {
		if ansi.StringWidth(line) != detailsTimeWidth {
			t.Fatalf("time line %d width = %d, want %d", index, ansi.StringWidth(line), detailsTimeWidth)
		}
		highlighted := ui.DashboardCurrentHourStyle.Render("   ")
		if index == middle {
			highlighted = ui.DashboardCurrentHourStyle.Render("12p")
			if plain := strings.TrimSpace(ansi.Strip(line)); plain != "now 12p" {
				t.Fatalf("current time label = %q, want now 12p", plain)
			}
		}
		if !strings.Contains(line, highlighted) {
			t.Fatalf("time line %d does not contain its full-height background: %q", index, line)
		}
	}
}

func assertDetailPanelsShareRow(t *testing.T, view string) {
	t.Helper()
	titles := []string{"Surf height", "Swell", "Wind", "Tide", "Temperature"}
	headerLine := ""
	borderLine := ""
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "╭") && strings.Count(line, "┬") == len(titles)-1 {
			borderLine = line
		}
		lastIndex := -1
		for _, title := range titles {
			index := strings.Index(line, title)
			if index < 0 || index <= lastIndex {
				lastIndex = -1
				break
			}
			lastIndex = index
		}
		if lastIndex >= 0 {
			headerLine = line
		}
	}
	if headerLine != "" && borderLine != "" {
		assertDetailTitlesCentered(t, headerLine, borderLine, titles)
		return
	}
	t.Fatalf("detail panel titles are not in one left-to-right row:\n%s", view)
}

func assertDetailTitlesCentered(t *testing.T, headerLine, borderLine string, titles []string) {
	t.Helper()
	boundaries := make([]int, 0, len(titles)+1)
	for index, character := range []rune(borderLine) {
		if character == '╭' || character == '┬' || character == '╮' {
			boundaries = append(boundaries, index)
		}
	}
	if len(boundaries) != len(titles)+1 {
		t.Fatalf("detail grid boundaries = %v, want %d", boundaries, len(titles)+1)
	}
	for index, title := range titles {
		byteIndex := strings.Index(headerLine, title)
		if byteIndex < 0 {
			t.Fatalf("detail title %q is missing from row %q", title, headerLine)
		}
		titleStart := len([]rune(headerLine[:byteIndex]))
		titleCenter := float64(titleStart) + float64(len([]rune(title))-1)/2
		cellCenter := float64(boundaries[index]+boundaries[index+1]) / 2
		if difference := titleCenter - cellCenter; difference < -1 || difference > 1 {
			t.Fatalf("detail title %q center = %.1f, want cell center %.1f", title, titleCenter, cellCenter)
		}
	}
}
