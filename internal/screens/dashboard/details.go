package dashboard

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"surfista/internal/surf"
	"surfista/internal/ui"
)

const (
	detailsDialogWidth       = 118
	detailsDialogFrame       = 4
	detailsColumnCount       = 5
	detailsTimeWidth         = 8
	detailsCellContentHeight = 4
	detailsForecastRowHeight = detailsCellContentHeight + 2
	detailsDialogFixedHeight = 11
)

type forecastDetailRow struct {
	forecast  surf.ForecastSlot
	details   surf.ForecastDetailSlot
	hasDetail bool
	timeLabel string
	current   bool
}

type swellColumnLayout struct {
	heightWidth    int
	periodWidth    int
	directionWidth int
}

func (m Model) detailsOverlay(dashboard string) string {
	dialog := m.detailsDialog()
	if m.terminalWidth <= 0 || m.terminalHeight <= 0 {
		return lipgloss.JoinVertical(lipgloss.Center, dashboard, "", dialog)
	}

	headerHeight := m.detailsHeaderHeight()
	availableHeight := max(0, m.terminalHeight-headerHeight)
	canvas := lipgloss.NewCanvas(m.terminalWidth, m.terminalHeight)
	compositor := lipgloss.NewCompositor(
		lipgloss.NewLayer(dashboard).Z(0),
		lipgloss.NewLayer(dialog).
			X(max(0, (m.terminalWidth-lipgloss.Width(dialog))/2)).
			Y(headerHeight+max(0, (availableHeight-lipgloss.Height(dialog))/2)).
			Z(1),
	)
	return canvas.Compose(compositor).Render()
}

func (m Model) detailsDialog() string {
	width := detailsDialogWidth
	if m.terminalWidth > 0 {
		width = max(1, min(width, m.terminalWidth-2))
	}
	contentWidth := max(1, width-detailsDialogFrame)
	cellWidth := detailCellWidth(contentWidth)
	rows := m.detailsForecastRows()
	visibleCount := m.detailsVisibleRowCount(len(rows))
	maxOffset := max(0, len(rows)-visibleCount)
	offset := min(max(0, m.detailsScroll), maxOffset)
	end := min(len(rows), offset+visibleCount)

	title := m.detailsTitleLine(contentWidth)
	header := m.detailCategoryHeader(contentWidth, cellWidth)

	topIndicator := ""
	if offset > 0 {
		topIndicator = ui.DashboardScrollIndicatorStyle.Width(contentWidth).Render("↑")
	}
	bottomIndicator := ""
	if end < len(rows) {
		bottomIndicator = ui.DashboardScrollIndicatorStyle.Width(contentWidth).Render("↓")
	}

	rowViews := make([]string, 0, visibleCount)
	swellLayout := swellColumnsForRows(rows)
	for _, row := range rows[offset:end] {
		rowViews = append(rowViews, m.detailForecastRow(row, contentWidth, cellWidth, swellLayout))
	}
	if len(rowViews) == 0 {
		rowViews = append(rowViews, fitHeight(
			ui.DashboardEmptyStyle.Width(contentWidth).Render("No forecast time slots available."),
			detailsForecastRowHeight,
		))
	}
	rowBlock := lipgloss.JoinVertical(lipgloss.Left, rowViews...)

	status := m.detailsStatus(contentWidth)
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		"",
		header,
		topIndicator,
		rowBlock,
		bottomIndicator,
		"",
		status,
	)
	return ui.DashboardDetailDialogStyle.Width(width).Render(content)
}

func (m Model) detailsStatus(width int) string {
	actions := []string{"←/→/h/l day", "↑/↓/j/k"}
	if _, ok := m.detailsBrowserURL(); ok {
		actions = append(actions, "u open")
	}
	actions = append(actions, "esc close")
	status := ui.DashboardDetailHelpStyle.Render(strings.Join(actions, " • "))
	status = ansi.Truncate(status, width, "")
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(status)
}

func (m Model) detailsTitleLine(width int) string {
	title := ui.DashboardDetailTitleStyle.Render(ansi.Truncate(m.detailsSpot.Name, width, ""))
	freshness := m.detailsFreshness()
	canvas := lipgloss.NewCanvas(width, 1)
	line := canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(title).X(max(0, (width-lipgloss.Width(title))/2)),
		lipgloss.NewLayer(freshness).X(max(0, width-lipgloss.Width(freshness)-1)),
	)).Render()
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(line)
}

func (m Model) detailsFreshness() string {
	state := m.details[m.detailsSpot.ID]
	label := ""
	failed := false
	switch {
	case state.loading && !state.usable():
		label = m.refreshSpinner.View() + " loading details…"
	case state.loading:
		freshness := formatForecastAge(m.now(), state.updatedAt)
		if state.fetched {
			freshness = "now"
		}
		label = m.refreshSpinner.View() + " updated " + freshness
	case state.err != nil && state.usable():
		failed = true
		label = "failed"
		if !state.updatedAt.IsZero() {
			age := formatForecastAge(m.now(), state.updatedAt)
			label += " · " + strings.TrimSuffix(age, " ago")
		}
	case state.err != nil:
		failed = true
		label = "unavailable"
	case state.usable():
		label = "updated"
		if !state.updatedAt.IsZero() {
			label += " " + formatForecastAge(m.now(), state.updatedAt)
		}
		if state.fetched {
			label = "updated now"
		}
	}
	if label == "" {
		return ""
	}
	styled := ui.DashboardSubtitleStyle.Render(label)
	if failed {
		styled = ui.ErrorStyle.Render("●") + " " + styled
	}
	return styled
}

func (m Model) detailCategoryHeader(contentWidth, cellWidth int) string {
	titles := detailCategoryTitles(cellWidth)
	cells := make([]string, 0, len(titles))
	for _, title := range titles {
		cells = append(cells, lipgloss.NewStyle().Width(cellWidth).Align(lipgloss.Center).Render(
			ui.DashboardDetailLabelStyle.Render(ansi.Truncate(title, cellWidth, "")),
		))
	}
	header := strings.Repeat(" ", detailsTimeWidth+2) + strings.Join(cells, " ")
	return lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(header)
}

func detailCategoryTitles(cellWidth int) []string {
	full := []string{"Surf height", "Swell", "Wind", "Tide", "Temperature"}
	for _, title := range full {
		if ansi.StringWidth(title) >= cellWidth {
			return []string{"Surf", "Swell", "Wind", "Tide", "Temp"}
		}
	}
	return full
}

func (m Model) detailForecastRow(row forecastDetailRow, contentWidth, cellWidth int, swellLayout swellColumnLayout) string {
	detailsState := m.details[m.detailsSpot.ID]
	detailsLoading := detailsState.loading && !detailsState.usable()
	cells := [][]string{
		surfHeightRowLines(row.forecast, cellWidth),
		swellRowLines(row.forecast, cellWidth, swellLayout),
		windDetailLines(row.details, row.hasDetail, detailsLoading, detailsState.details.Units),
		tideDetailLines(detailsState.details, row.forecast.Timestamp, detailsLoading, cellWidth),
		temperatureDetailLines(row.details, row.hasDetail, detailsLoading, detailsState.details.Units),
	}

	gridLines := make([]string, 0, detailsForecastRowHeight)
	gridLines = append(gridLines, detailGridBorder("╭", "┬", "╮", cellWidth))
	for lineIndex := range detailsCellContentHeight {
		gridLines = append(gridLines, detailGridLine(cells, lineIndex, cellWidth))
	}
	gridLines = append(gridLines, detailGridBorder("╰", "┴", "╯", cellWidth))

	timeLines := detailTimeLabelLines(row)

	rowLines := make([]string, detailsForecastRowHeight)
	for index := range rowLines {
		rowLines[index] = timeLines[index] + " " + gridLines[index]
	}
	return lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(strings.Join(rowLines, "\n"))
}

func (m Model) detailsForecastRows() []forecastDetailRow {
	forecastState, ok := m.forecasts[m.detailsSpot.ID]
	if !ok || !forecastState.usable() {
		return nil
	}
	detailsState := m.details[m.detailsSpot.ID]
	detailsByTimestamp := make(map[int64]surf.ForecastDetailSlot, len(detailsState.details.Slots))
	for _, slot := range detailsState.details.Slots {
		detailsByTimestamp[slot.Timestamp.Unix()] = slot
	}
	now := m.now()
	forecastSlots := slotsForLocalDayOffset(forecastState.forecast, now, m.forecastDayOffset)
	rows := make([]forecastDetailRow, 0, len(forecastSlots))
	localNow := now.UTC().Add(forecastState.forecast.UTCOffset)
	selectedDate := localDate(now, forecastState.forecast.UTCOffset).AddDate(0, 0, m.forecastDayOffset)
	ny, nm, nd := localNow.Date()
	dy, dm, dd := selectedDate.Date()
	currentHour := now.Hour()
	for _, slot := range forecastSlots {
		localSlot := slot.Timestamp.UTC().Add(forecastState.forecast.UTCOffset)
		hour := localSlot.Hour()
		sy, sm, sd := localSlot.Date()
		current := m.forecastDayOffset == 0 && sy == ny && sm == nm && sd == nd && localSlot.Hour() == currentHour
		if sy != dy || sm != dm || sd != dd {
			hour = 24
		}
		detail, hasDetail := detailsByTimestamp[slot.Timestamp.Unix()]
		rows = append(rows, forecastDetailRow{
			forecast:  slot,
			details:   detail,
			hasDetail: hasDetail,
			timeLabel: formatDashboardHour(hour),
			current:   current,
		})
	}
	return rows
}

func (m Model) detailsVisibleRowCount(total int) int {
	if total <= 0 {
		return 1
	}
	if m.terminalHeight <= 0 {
		return total
	}
	availableHeight := max(0, m.terminalHeight-m.detailsHeaderHeight())
	return max(1, min(total, (availableHeight-detailsDialogFixedHeight)/detailsForecastRowHeight))
}

func (m Model) detailsHeaderHeight() int {
	if m.terminalHeight <= 0 {
		return 0
	}
	return lipgloss.Height(m.forecastHeader(m.contentWidth()))
}

func (m *Model) resetDetailsScroll() {
	rows := m.detailsForecastRows()
	visible := m.detailsVisibleRowCount(len(rows))
	currentIndex := 0
	for index, row := range rows {
		if row.current {
			currentIndex = index
			break
		}
	}
	m.detailsScroll = min(max(0, currentIndex-visible/2), max(0, len(rows)-visible))
}

func (m *Model) clampDetailsScroll() {
	rows := m.detailsForecastRows()
	visible := m.detailsVisibleRowCount(len(rows))
	m.detailsScroll = min(max(0, m.detailsScroll), max(0, len(rows)-visible))
}

func detailCellWidth(contentWidth int) int {
	gridSpace := max(detailsColumnCount*3+detailsColumnCount+1, contentWidth-detailsTimeWidth-1)
	return max(3, (gridSpace-(detailsColumnCount+1))/detailsColumnCount)
}

func detailGridBorder(left, divider, right string, cellWidth int) string {
	return ui.DashboardBorderStyle.Render(
		left + strings.Join(repeatString(strings.Repeat("─", cellWidth), detailsColumnCount), divider) + right,
	)
}

func detailGridLine(cells [][]string, lineIndex, cellWidth int) string {
	var line strings.Builder
	line.WriteString(ui.DashboardBorderStyle.Render("│"))
	for index, cell := range cells {
		value := ""
		if lineIndex < len(cell) {
			value = cell[lineIndex]
		}
		line.WriteString(padDetailCell(value, cellWidth))
		if index < len(cells)-1 {
			line.WriteString(ui.DashboardBorderStyle.Render("│"))
		}
	}
	line.WriteString(ui.DashboardBorderStyle.Render("│"))
	return line.String()
}

func detailTimeLabelLines(row forecastDetailRow) []string {
	lines := make([]string, detailsForecastRowHeight)
	middle := detailsForecastRowHeight / 2
	if !row.current {
		for index := range lines {
			value := ""
			if index == middle {
				value = row.timeLabel
			}
			lines[index] = lipgloss.NewStyle().Width(detailsTimeWidth).Align(lipgloss.Right).Render(value)
		}
		return lines
	}

	const timeBlockWidth = 3
	labelWidth := max(0, detailsTimeWidth-timeBlockWidth)
	for index := range lines {
		left := strings.Repeat(" ", labelWidth)
		timeValue := ""
		if index == middle {
			left = lipgloss.NewStyle().Width(labelWidth).Align(lipgloss.Right).Render(
				ui.DashboardNowStyle.Render("now") + " ",
			)
			timeValue = row.timeLabel
		}
		timeBlock := lipgloss.NewStyle().Width(timeBlockWidth).Align(lipgloss.Center).Render(timeValue)
		lines[index] = left + ui.DashboardCurrentHourStyle.Render(timeBlock)
	}
	return lines
}

func padDetailCell(value string, width int) string {
	value = ansi.Truncate(value, width, "")
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(value)
}

func repeatString(value string, count int) []string {
	values := make([]string, count)
	for index := range values {
		values[index] = value
	}
	return values
}

func surfHeightRowLines(slot surf.ForecastSlot, width int) []string {
	rating := uppercaseFirst(strings.ToLower(slot.Rating))
	if ansi.StringWidth(rating) >= width {
		rating = compactRating(slot.Rating, width)
	}
	lines := []string{
		ui.DashboardDetailValueStyle.Render(formatDetailHeight(slot.SurfHeight)),
		ui.DashboardRating(rating, slot.Rating),
	}
	if relation := strings.TrimSpace(slot.SurfHeight.HumanRelation); relation != "" {
		relation = uppercaseFirst(strings.ToLower(relation))
		available := detailsCellContentHeight - len(lines)
		lines = append(lines, compactSurfRelation(relation, width, available)...)
	}
	return lines[:min(len(lines), detailsCellContentHeight)]
}

func compactSurfRelation(relation string, width, maxLines int) []string {
	lines := wrapDetailText(relation, width)
	if len(lines) <= maxLines {
		return lines
	}

	words := strings.Fields(relation)
	for index, word := range words {
		if strings.EqualFold(word, "overhead") {
			words[index] = "OH"
		}
	}
	lines = wrapDetailText(strings.Join(words, " "), width)
	return lines[:min(len(lines), maxLines)]
}

func swellColumnsForRows(rows []forecastDetailRow) swellColumnLayout {
	var layout swellColumnLayout
	for _, row := range rows {
		visibleSwells := row.forecast.Swells[:min(len(row.forecast.Swells), detailsCellContentHeight)]
		for _, swell := range visibleSwells {
			layout.heightWidth = max(layout.heightWidth, ansi.StringWidth(formatDetailNumber(swell.Height)+"′"))
			layout.periodWidth = max(layout.periodWidth, ansi.StringWidth(formatDetailNumber(swell.Period)+"s"))
			layout.directionWidth = max(layout.directionWidth, ansi.StringWidth(compassDirection(swell.Direction)))
		}
	}
	return layout
}

func swellRowLines(slot surf.ForecastSlot, width int, layout swellColumnLayout) []string {
	if len(slot.Swells) == 0 {
		return []string{ui.Muted("unavailable")}
	}

	type swellValues struct {
		height    string
		period    string
		direction string
	}
	visibleSwells := slot.Swells[:min(len(slot.Swells), detailsCellContentHeight)]
	values := make([]swellValues, 0, len(visibleSwells))
	for _, swell := range visibleSwells {
		value := swellValues{
			height:    formatDetailNumber(swell.Height) + "′",
			period:    formatDetailNumber(swell.Period) + "s",
			direction: compassDirection(swell.Direction),
		}
		values = append(values, value)
	}

	lines := make([]string, 0, len(values))
	for _, value := range values {
		line := lipgloss.NewStyle().Width(layout.heightWidth).Align(lipgloss.Right).Render(value.height) + " " +
			lipgloss.NewStyle().Width(layout.periodWidth).Align(lipgloss.Right).Render(value.period) + " " +
			lipgloss.NewStyle().Width(layout.directionWidth).Align(lipgloss.Left).Render(value.direction)
		lines = append(lines, ansi.Truncate(line, width, ""))
	}
	return lines
}

func windDetailLines(slot surf.ForecastDetailSlot, available, loading bool, units surf.ForecastUnits) []string {
	if loading {
		return []string{ui.Muted("loading…")}
	}
	if !available {
		return []string{ui.Muted("unavailable")}
	}
	unit := normalizedUnit(units.WindSpeed, "kts")
	lines := []string{ui.DashboardDetailValueStyle.Render(fmt.Sprintf("%s %s %s",
		formatDetailNumber(slot.Wind.Speed), unit, compassDirection(slot.Wind.Direction)))}
	if slot.Wind.Gust > 0 {
		lines = append(lines, fmt.Sprintf("gusts %s %s", formatDetailNumber(slot.Wind.Gust), unit))
	}
	if directionType := strings.TrimSpace(slot.Wind.DirectionType); directionType != "" {
		lines = append(lines, strings.ReplaceAll(strings.ToLower(directionType), "_", "-"))
	}
	return lines
}

func tideDetailLines(details surf.ForecastDetails, at time.Time, loading bool, width int) []string {
	if loading {
		return []string{ui.Muted("loading…")}
	}
	if len(details.Tides) == 0 {
		return []string{ui.Muted("unavailable")}
	}

	previous, next, height, hasHeight := tidePosition(details.Tides, at)
	unit := tideHeightUnit(details.Units.TideHeight)
	phase := "steady"
	if next != nil {
		switch next.Type {
		case "HIGH":
			phase = "rising"
		case "LOW":
			phase = "falling"
		}
	}
	lines := make([]string, 0, 3)
	if hasHeight {
		heightLabel := formatDetailNumber(height) + unit
		current := heightLabel + " " + phase
		if ansi.StringWidth(current) >= width {
			compactPhase := map[string]string{"rising": "rise", "falling": "fall", "steady": "still"}[phase]
			current = heightLabel + " " + compactPhase
		}
		lines = append(lines, ui.DashboardDetailValueStyle.Render(current))
	}
	if previous != nil {
		lines = append(lines, formatTideEvent(*previous, details.UTCOffset, unit, width))
	}
	if next != nil {
		lines = append(lines, formatTideEvent(*next, details.UTCOffset, unit, width))
	}
	return lines
}

func tidePosition(points []surf.TidePoint, at time.Time) (previousEvent, nextEvent *surf.TidePoint, height float64, hasHeight bool) {
	var before, after *surf.TidePoint
	for index := range points {
		point := &points[index]
		if !point.Timestamp.After(at) {
			before = point
			if point.Type == "HIGH" || point.Type == "LOW" {
				previousEvent = point
			}
			continue
		}
		if after == nil {
			after = point
		}
		if point.Type == "HIGH" || point.Type == "LOW" {
			nextEvent = point
			break
		}
	}

	switch {
	case before != nil && after != nil:
		span := after.Timestamp.Sub(before.Timestamp)
		progress := 0.0
		if span > 0 {
			progress = float64(at.Sub(before.Timestamp)) / float64(span)
		}
		return previousEvent, nextEvent, before.Height + ((after.Height - before.Height) * progress), true
	case before != nil:
		return previousEvent, nextEvent, before.Height, true
	case after != nil:
		return previousEvent, nextEvent, after.Height, true
	default:
		return previousEvent, nextEvent, 0, false
	}
}

func formatTideEvent(point surf.TidePoint, offset time.Duration, unit string, width int) string {
	local := point.Timestamp.UTC().Add(offset)
	timeLabel := strings.TrimSuffix(local.Format("3:04pm"), "m")
	eventLabel := strings.ToLower(point.Type)
	switch point.Type {
	case "LOW":
		eventLabel = "l"
	case "HIGH":
		eventLabel = "h"
	}
	heightLabel := formatDetailNumber(point.Height) + unit
	full := fmt.Sprintf("%s %s %s", eventLabel, timeLabel, heightLabel)
	if ansi.StringWidth(full) < width {
		return full
	}

	return eventLabel + " " + timeLabel
}

func temperatureDetailLines(slot surf.ForecastDetailSlot, available, loading bool, units surf.ForecastUnits) []string {
	if loading {
		return []string{ui.Muted("loading…")}
	}
	if !available || slot.Temperature == nil {
		return []string{ui.Muted("unavailable")}
	}
	unit := strings.ToUpper(strings.TrimSpace(units.Temperature))
	if unit == "" {
		unit = "F"
	}
	return []string{ui.DashboardDetailValueStyle.Render(fmt.Sprintf("%s°%s", formatDetailNumber(*slot.Temperature), unit))}
}

func normalizedUnit(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
}

func tideHeightUnit(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "FT", "FEET":
		return "′"
	case "M", "METERS", "METRES":
		return "m"
	default:
		return "′"
	}
}

func compassDirection(degrees float64) string {
	directions := [...]string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	index := int(math.Round(math.Mod(degrees+360, 360)/22.5)) % len(directions)
	return directions[index]
}

func formatDetailNumber(value float64) string {
	return strconv.FormatFloat(math.Round(value*10)/10, 'f', -1, 64)
}

func formatDetailHeight(height surf.SurfHeight) string {
	plus := ""
	if height.Plus {
		plus = "+"
	}
	return fmt.Sprintf("%s–%s′%s", formatDetailNumber(height.Min), formatDetailNumber(height.Max), plus)
}

func wrapDetailText(value string, width int) []string {
	words := strings.Fields(value)
	if len(words) == 0 || width <= 0 {
		return nil
	}

	lines := make([]string, 0, len(words))
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) <= width {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	return append(lines, current)
}

func uppercaseFirst(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
	return string(runes)
}
