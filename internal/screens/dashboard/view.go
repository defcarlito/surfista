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
	maxDashboardWidth       = 114
	pageMargin              = 2
	maxSlotWidth            = 10
	gridBorderWidth         = 10
	confirmationDialogWidth = 48
	confirmationDialogFrame = 6
	spaciousLayoutMinHeight = 13
)

var dashboardHours = [...]int{0, 3, 6, 9, 12, 15, 18, 21, 24}

const (
	dashboardBrowseHelp = "←/→/h/l day • ↑/↓/j/k • s sort • / search • q quit"
	dashboardSelectHelp = "←/→/h/l day • ↑/↓/j/k • s sort • enter • x remove • esc • q quit"
	dashboardURLHelp    = "←/→/h/l day • ↑/↓/j/k • s sort • enter • u open • x remove • esc • q quit"
)

func (m Model) View() string {
	width := m.contentWidth()
	header := m.dashboardHeader(width)
	footer := m.dashboardFooter(width)
	sections := []string{header}
	if m.terminalHeight > 0 {
		bodyHeight := max(0, m.terminalHeight-lipgloss.Height(header)-lipgloss.Height(footer))
		if bodyHeight > 0 {
			sections = append(sections, m.dashboardBody(width, bodyHeight))
		}
		sections = append(sections, footer)
	} else {
		sections = append(sections, m.dashboardBody(width, 0), footer)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	if m.terminalWidth > 0 {
		content = lipgloss.PlaceHorizontal(m.terminalWidth, lipgloss.Center, content)
	}
	if m.confirmRemoval {
		return m.confirmationOverlay(content, m.removalDialog())
	}
	if m.confirmRefresh {
		return m.confirmationOverlay(content, m.refreshDialog())
	}
	if m.detailsOpen {
		return m.detailsOverlay(content)
	}
	return content
}

func (m Model) dashboardHeader(width int) string {
	if m.terminalHeight > 0 && m.terminalHeight < spaciousLayoutMinHeight {
		return lipgloss.JoinVertical(lipgloss.Left, m.forecastHeader(width), m.sortStatus(width))
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.forecastHeader(width), "", m.sortStatus(width))
}

func (m Model) dashboardFooter(width int) string {
	browseHelp := dashboardBrowseHelp
	selectHelp := dashboardSelectHelp
	urlHelp := dashboardURLHelp
	if m.canRefresh() {
		browseHelp = strings.Replace(browseHelp, "s sort", "s sort • r refresh", 1)
		selectHelp = strings.Replace(selectHelp, "s sort", "s sort • r refresh", 1)
		urlHelp = strings.Replace(urlHelp, "s sort", "s sort • r refresh", 1)
	}

	helpText := browseHelp
	if m.HasSelection() {
		helpText = selectHelp
	}
	if m.CanOpenSelectionURL() {
		helpText = urlHelp
	}
	if m.detailsOpen {
		helpText = ""
	}
	help := ui.DashboardHelpStyle.Width(width).Render(helpText)
	maxHelpHeight := max(
		lipgloss.Height(ui.DashboardHelpStyle.Width(width).Render(browseHelp)),
		lipgloss.Height(ui.DashboardHelpStyle.Width(width).Render(selectHelp)),
		lipgloss.Height(ui.DashboardHelpStyle.Width(width).Render(urlHelp)),
	)
	help = fitHeight(help, maxHelpHeight)
	return lipgloss.JoinVertical(lipgloss.Left, help, "")
}

func (m Model) sortStatus(width int) string {
	render := func(mode SortMode) string {
		label := ui.DashboardSortStyle.Render("sorting by: " + mode.label())
		if mode == SortConditionHighToLow {
			context := "best right now"
			if m.forecastDayOffset > 0 {
				context = "best overall for " + formatDashboardDate(m.dashboardForecastDate(m.forecastDayOffset))
			}
			label += " " + ui.DashboardSubtitleStyle.Render(context)
		}
		return lipgloss.NewStyle().Width(width).Render(label)
	}
	status := render(m.sortMode)
	maxHeight := max(lipgloss.Height(render(SortTimeAdded)), lipgloss.Height(render(SortConditionHighToLow)))
	if m.detailsOpen {
		status = ""
	}
	return fitHeight(status, maxHeight)
}

func (m Model) dashboardBody(width, height int) string {
	sections := make([]string, 0, 3)
	remainingHeight := height

	if m.loadErr != nil {
		loadError := ui.ErrorStyle.Width(width).Render("Could not load favorite locations: " + m.loadErr.Error())
		sections = append(sections, loadError)
		if height > 0 {
			remainingHeight -= lipgloss.Height(loadError)
		}
		if len(m.spots) > 0 && (height <= 0 || remainingHeight > 0) {
			sections = append(sections, "")
			if height > 0 {
				remainingHeight--
			}
		}
	}

	if len(m.spots) == 0 {
		sections = append(sections, ui.DashboardEmptyStyle.Width(width).Render("No favorite surf spots yet."))
	} else if height <= 0 {
		sections = append(sections, m.allLocationCards(width))
	} else if remainingHeight > 0 {
		sections = append(sections, m.locationViewport(width, remainingHeight))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	if height > 0 {
		return fitHeight(content, height)
	}
	return content
}

func (m Model) forecastHeader(width int) string {
	slotWidth := dashboardSlotWidth(width)
	dateCells := make([]string, 0, len(dashboardHours))
	headerCells := make([]string, 0, len(dashboardHours))
	nowCells := make([]string, 0, len(dashboardHours))
	startDate := m.dashboardForecastDate(m.forecastDayOffset)
	endDate := startDate.AddDate(0, 0, 1)
	for index, hour := range dashboardHours {
		date := ""
		if index == 0 {
			date = formatDashboardDate(startDate)
		} else if index == len(dashboardHours)-1 {
			date = formatDashboardDate(endDate)
		}
		dateCells = append(dateCells, tableCell(ui.DashboardSubtitleStyle.Render(date), slotWidth))

		if m.forecastDayOffset == 0 && isCurrentDashboardHour(hour, m.now()) {
			headerCells = append(headerCells, ui.DashboardCurrentHourStyle.Width(slotWidth).MaxWidth(slotWidth).Render(formatDashboardHour(hour)))
			nowCells = append(nowCells, ui.DashboardNowStyle.Width(slotWidth).MaxWidth(slotWidth).Render("now"))
			continue
		}
		headerCells = append(headerCells, tableCell(formatDashboardHour(hour), slotWidth))
		nowCells = append(nowCells, tableCell("", slotWidth))
	}
	dateRow := m.forecastAnnotationRow(" "+strings.Join(dateCells, " ")+" ", slotWidth)
	header := " " + strings.Join(headerCells, " ") + " "
	nowRow := " " + strings.Join(nowCells, " ") + " "
	cardWidth := slotWidth*len(dashboardHours) + gridBorderWidth
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(
		lipgloss.NewStyle().Width(cardWidth).Render(lipgloss.JoinVertical(lipgloss.Left, dateRow, header, nowRow)),
	)
}

func (m Model) forecastAnnotationRow(dateRow string, slotWidth int) string {
	cardWidth := slotWidth*len(dashboardHours) + gridBorderWidth
	day, utcOffset, ok := m.headerSunlightDay()
	if !ok {
		return dateRow
	}

	layers := []*lipgloss.Layer{lipgloss.NewLayer(dateRow).Z(0)}
	for _, marker := range []struct {
		timestamp time.Time
		arrow     string
	}{
		{timestamp: day.Sunrise, arrow: "↑"},
		{timestamp: day.Sunset, arrow: "↓"},
	} {
		if marker.timestamp.IsZero() {
			continue
		}
		label := marker.arrow + formatDashboardSunlightTime(marker.timestamp, utcOffset)
		x := dashboardTimePosition(marker.timestamp, utcOffset, slotWidth) - sunlightLabelPivot(label)
		x = max(0, min(x, cardWidth-ansi.StringWidth(label)))
		layers = append(layers, lipgloss.NewLayer(ui.DashboardSubtitleStyle.Render(label)).X(x).Z(1))
	}

	return lipgloss.NewCanvas(cardWidth, 1).
		Compose(lipgloss.NewCompositor(layers...)).
		Render()
}

func sunlightLabelPivot(label string) int {
	digitsSeen := 0
	for index := len(label) - 1; index >= 0; index-- {
		if label[index] >= '0' && label[index] <= '9' {
			digitsSeen++
			if digitsSeen == 2 {
				return ansi.StringWidth(label[:index])
			}
		}
	}
	return 0
}

func (m Model) headerSunlightDay() (surf.SunlightDay, time.Duration, bool) {
	spotID := m.forecastHeaderSpotID()
	if spotID == "" {
		return surf.SunlightDay{}, 0, false
	}

	forecast := m.forecasts[spotID].forecast
	if len(forecast.Slots) == 0 {
		return surf.SunlightDay{}, 0, false
	}
	if day, ok := sunlightForLocalDay(m.details[spotID].details, m.now(), forecast.UTCOffset, m.forecastDayOffset); ok {
		return day, forecast.UTCOffset, true
	}
	return surf.SunlightDay{}, 0, false
}

func dashboardTimePosition(timestamp time.Time, utcOffset time.Duration, slotWidth int) int {
	local := timestamp.UTC().Add(utcOffset)
	minutes := float64(local.Hour()*60+local.Minute()) + float64(local.Second())/60
	startCenter := 1 + float64(slotWidth-1)/2
	threeHourWidth := float64(slotWidth + 1)
	return int(math.Round(startCenter + minutes/(3*60)*threeHourWidth))
}

func formatDashboardSunlightTime(timestamp time.Time, utcOffset time.Duration) string {
	local := timestamp.UTC().Add(utcOffset)
	hour := local.Hour()
	suffix := "a"
	if hour >= 12 {
		suffix = "p"
	}
	hour %= 12
	if hour == 0 {
		hour = 12
	}
	return fmt.Sprintf("%d:%02d%s", hour, local.Minute(), suffix)
}

func dashboardSlotWidth(width int) int {
	return max(1, min(maxSlotWidth, (width-gridBorderWidth)/len(dashboardHours)))
}

func (m Model) allLocationCards(width int) string {
	slotWidth := dashboardSlotWidth(width)
	cards := make([]string, 0, len(m.spots))
	for index, spot := range m.spots {
		cards = append(cards, m.spotCard(spot, slotWidth, index == m.selectedIndex))
	}
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(strings.Join(cards, "\n\n"))
}

type locationWindow struct {
	start     int
	end       int
	showAbove bool
	showBelow bool
}

func (m Model) locationWindowFor(width, height int) locationWindow {
	start := min(max(0, m.scrollOffset), len(m.spots))
	window := locationWindow{start: start, end: start}
	window.showAbove = window.start > 0
	if height <= 0 || window.start >= len(m.spots) {
		return window
	}

	// Always reserve one row above and below the cards. Keeping these rows
	// stable prevents the cards from moving when either arrow disappears.
	available := height - 2
	if available <= 0 {
		window.showBelow = window.start < len(m.spots)
		return window
	}

	slotWidth := dashboardSlotWidth(width)
	used := 0
	for index := window.start; index < len(m.spots); index++ {
		separatorHeight := 0
		if index > window.start {
			separatorHeight = 1
		}
		cardHeight := lipgloss.Height(m.spotCard(m.spots[index], slotWidth, index == m.selectedIndex))
		if used+separatorHeight+cardHeight > available {
			if window.end == window.start {
				window.end = index + 1
			}
			break
		}
		used += separatorHeight + cardHeight
		window.end = index + 1
	}
	window.showBelow = window.end < len(m.spots)
	return window
}

func (m Model) locationViewport(width, height int) string {
	window := m.locationWindowFor(width, height)
	lines := make([]string, 0, height)
	topIndicator := ""
	if window.showAbove {
		topIndicator = ui.DashboardScrollIndicatorStyle.Width(width).Render("↑")
	}
	lines = append(lines, topIndicator)

	cardLines := make([]string, 0, height)
	if window.end > window.start {
		slotWidth := dashboardSlotWidth(width)
		cards := make([]string, 0, window.end-window.start)
		for index := window.start; index < window.end; index++ {
			cards = append(cards, m.spotCard(m.spots[index], slotWidth, index == m.selectedIndex))
		}
		cardsBlock := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(strings.Join(cards, "\n\n"))
		cardLines = strings.Split(cardsBlock, "\n")
	}

	cardAreaHeight := max(0, height-2)
	if len(cardLines) > cardAreaHeight {
		cardLines = cardLines[:cardAreaHeight]
	}
	topPadding := max(0, (cardAreaHeight-len(cardLines))/2)
	for range topPadding {
		lines = append(lines, "")
	}
	lines = append(lines, cardLines...)
	for len(lines) < height-1 {
		lines = append(lines, "")
	}
	if height > 1 {
		bottomIndicator := ""
		if window.showBelow {
			bottomIndicator = ui.DashboardScrollIndicatorStyle.Width(width).Render("↓")
		}
		lines = append(lines, bottomIndicator)
	}
	return fitHeight(strings.Join(lines, "\n"), height)
}

func fitHeight(content string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m Model) locationViewportHeight(width int) int {
	if m.terminalHeight <= 0 {
		return 0
	}
	height := m.terminalHeight - lipgloss.Height(m.dashboardHeader(width)) - lipgloss.Height(m.dashboardFooter(width))
	if m.loadErr != nil {
		loadError := ui.ErrorStyle.Width(width).Render("Could not load favorite locations: " + m.loadErr.Error())
		height -= lipgloss.Height(loadError) + 1
	}
	return max(0, height)
}

func (m *Model) clampScrollOffset() {
	if len(m.spots) == 0 {
		m.scrollOffset = 0
		return
	}
	m.scrollOffset = min(max(0, m.scrollOffset), len(m.spots)-1)
}

func (m *Model) ensureSelectedVisible() {
	m.clampScrollOffset()
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.spots) || m.terminalHeight <= 0 {
		return
	}
	if m.selectedIndex < m.scrollOffset {
		m.scrollOffset = m.selectedIndex
		return
	}

	width := m.contentWidth()
	height := m.locationViewportHeight(width)
	if height <= 0 {
		return
	}
	for m.scrollOffset < m.selectedIndex {
		window := m.locationWindowFor(width, height)
		if m.selectedIndex < window.end {
			return
		}
		m.scrollOffset++
	}
}

func (m Model) confirmationOverlay(dashboard, dialog string) string {
	if m.terminalWidth <= 0 || m.terminalHeight <= 0 {
		return lipgloss.JoinVertical(lipgloss.Center, dashboard, "", dialog)
	}

	canvas := lipgloss.NewCanvas(m.terminalWidth, m.terminalHeight)
	compositor := lipgloss.NewCompositor(
		lipgloss.NewLayer(dashboard).Z(0),
		lipgloss.NewLayer(dialog).
			X(max(0, (m.terminalWidth-lipgloss.Width(dialog))/2)).
			Y(max(0, (m.terminalHeight-lipgloss.Height(dialog))/2)).
			Z(1),
	)
	return canvas.Compose(compositor).Render()
}

func (m Model) removalDialog() string {
	width := confirmationDialogWidth
	if m.terminalWidth > 0 {
		width = max(1, min(width, m.terminalWidth-confirmationDialogFrame))
	}
	contentWidth := max(1, width-confirmationDialogFrame)

	status := ui.DashboardRemovalHelpStyle.Render("enter ") +
		ui.SuccessStyle.Render("remove") +
		ui.DashboardRemovalHelpStyle.Render(" • esc ") +
		ui.ErrorStyle.Render("cancel")
	if m.removing {
		status = ui.DashboardRemovalHelpStyle.Render("Removing…")
	} else if m.removalErr != nil {
		status = ui.ErrorStyle.Render("Could not remove: "+m.removalErr.Error()) +
			"\n" + ui.DashboardRemovalHelpStyle.Render("enter retry • esc ") +
			ui.ErrorStyle.Render("cancel")
	}
	status = lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(status)
	question := ui.DashboardRemovalBodyStyle.Render("Remove ") +
		ui.DashboardSpotStyle.Render(m.removalSpot.Name) +
		ui.DashboardRemovalBodyStyle.Render(" from tracked locations?")
	question = lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(question)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		question,
		"",
		status,
	)
	return ui.DashboardRemovalDialogStyle.Width(width).Render(content)
}

func (m Model) refreshDialog() string {
	width := confirmationDialogWidth
	if m.terminalWidth > 0 {
		width = max(1, min(width, m.terminalWidth-confirmationDialogFrame))
	}
	contentWidth := max(1, width-confirmationDialogFrame)

	status := ui.DashboardRemovalHelpStyle.Render("enter ") +
		ui.SuccessStyle.Render("refresh") +
		ui.DashboardRemovalHelpStyle.Render(" • esc ") +
		ui.ErrorStyle.Render("cancel")
	status = lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(status)
	question := ui.DashboardRemovalBodyStyle.Render("Refresh all forecast data?")
	question = lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(question)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		question,
		"",
		status,
	)
	return ui.DashboardRemovalDialogStyle.Width(width).Render(content)
}

func (m Model) contentWidth() int {
	if m.terminalWidth <= 0 {
		return maxDashboardWidth
	}
	return max(1, min(maxDashboardWidth, m.terminalWidth-(pageMargin*2)))
}

func (m Model) forecastTable(width int) string {
	return lipgloss.JoinVertical(lipgloss.Left, m.forecastHeader(width), "", m.allLocationCards(width))
}

func (m Model) spotCard(spot surf.Spot, slotWidth int, selected bool) string {
	state := m.forecasts[spot.ID]
	innerWidth := slotWidth*len(dashboardHours) + len(dashboardHours) - 1
	name := m.spotNameLine(spot.Name, innerWidth, state, m.spotFetching(spot.ID))

	if state.loading && !state.usable() {
		return statusCard(name, ui.Muted("Loading forecast…"), innerWidth, selected)
	}
	if state.err != nil && !state.usable() {
		return statusCard(name, ui.Error("Forecast temporarily unavailable."), innerWidth, selected)
	}

	now := m.now()
	slots := slotsByHourForDay(state.forecast, now, m.forecastDayOffset)
	if len(slots) == 0 {
		return statusCard(name, ui.Muted("No forecast available for this day."), innerWidth, selected)
	}

	ratings := make([]string, 0, len(dashboardHours))
	heights := make([]string, 0, len(dashboardHours))
	for _, hour := range dashboardHours {
		slot, ok := slots[hour]
		if !ok {
			ratings = append(ratings, tableCell("—", slotWidth))
			heights = append(heights, tableCell("—", slotWidth))
			continue
		}
		compact := compactRating(slot.Rating, slotWidth)
		ratings = append(ratings, tableCell(ui.DashboardRating(compact, slot.Rating), slotWidth))
		heights = append(heights, tableCell(compactHeight(slot.SurfHeight, slotWidth), slotWidth))
	}

	return strings.Join([]string{
		solidBorder("╭", "╮", innerWidth, selected),
		borderedLine(name, selected),
		segmentedBorder("├", "┬", "┤", slotWidth, selected),
		segmentedLine(ratings, selected),
		segmentedLine(heights, selected),
		segmentedBorder("╰", "┴", "╯", slotWidth, selected),
	}, "\n")
}

func (m Model) spotNameLine(name string, width int, state forecastState, fetching bool) string {
	styledName := ui.DashboardSpotStyle.Render(ansi.Truncate(name, width, ""))
	displayUpdatedAt := state.updatedAt
	if fetching && state.fetchDisplaySet {
		displayUpdatedAt = state.fetchDisplayAt
	}
	if displayUpdatedAt.IsZero() {
		if fetching {
			return m.spotNameLineWithFreshness(styledName, width, m.refreshSpinner.View(), false)
		}
		return lipgloss.NewStyle().Width(width).MaxWidth(width).Align(lipgloss.Center).Render(styledName)
	}

	freshness := "updated " + formatForecastAge(m.now(), displayUpdatedAt)
	if state.fetched && !state.loading && state.err == nil && !state.refreshFailed && !fetching {
		freshness = "updated now"
	}
	if fetching {
		freshness = m.refreshSpinner.View() + " " + freshness
	}
	return m.spotNameLineWithFreshness(styledName, width, freshness, state.refreshFailed && !fetching)
}

func (m Model) spotNameLineWithFreshness(styledName string, width int, freshness string, refreshFailed bool) string {
	styledFreshness := ui.DashboardSubtitleStyle.Render(freshness)
	if refreshFailed {
		styledFreshness = ui.ErrorStyle.Render("●") + " " + styledFreshness
	}
	canvas := lipgloss.NewCanvas(width, 1)
	line := canvas.Compose(lipgloss.NewCompositor(
		lipgloss.NewLayer(styledName).X(max(0, (width-lipgloss.Width(styledName))/2)),
		lipgloss.NewLayer(styledFreshness).X(max(0, width-lipgloss.Width(styledFreshness)-1)),
	)).Render()
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(line)
}

func formatForecastAge(now, updatedAt time.Time) string {
	age := now.Sub(updatedAt)
	if age < 0 {
		age = 0
	}
	switch {
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", max(1, int(age/time.Minute)))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(age/time.Hour))
	default:
		return fmt.Sprintf("%dd ago", int(age/(24*time.Hour)))
	}
}

func statusCard(name, status string, innerWidth int, selected bool) string {
	centeredStatus := lipgloss.NewStyle().Width(innerWidth).MaxWidth(innerWidth).Align(lipgloss.Center).Render(status)
	return strings.Join([]string{
		solidBorder("╭", "╮", innerWidth, selected),
		borderedLine(name, selected),
		solidBorder("├", "┤", innerWidth, selected),
		borderedLine(centeredStatus, selected),
		solidBorder("╰", "╯", innerWidth, selected),
	}, "\n")
}

func borderedLine(content string, selected bool) string {
	return outerBorder("│", selected) + content + outerBorder("│", selected)
}

func segmentedLine(cells []string, selected bool) string {
	var line strings.Builder
	for _, cell := range cells {
		line.WriteString(gridBorder("│", selected))
		line.WriteString(cell)
	}
	line.WriteString(gridBorder("│", selected))
	return line.String()
}

func segmentedBorder(left, divider, right string, slotWidth int, selected bool) string {
	if selected {
		var outline strings.Builder
		for index := range dashboardHours {
			if index == 0 {
				outline.WriteString(left)
			} else {
				outline.WriteString(divider)
			}
			outline.WriteString(strings.Repeat("─", slotWidth))
		}
		outline.WriteString(right)
		return ui.DashboardSelectedBorderStyle.Render(outline.String())
	}

	var line strings.Builder
	for index := range dashboardHours {
		junction := divider
		if index == 0 {
			junction = left
		}
		line.WriteString(border(junction))
		line.WriteString(border(strings.Repeat("─", slotWidth)))
	}
	line.WriteString(border(right))
	return line.String()
}

func solidBorder(left, right string, innerWidth int, selected bool) string {
	if selected {
		return ui.DashboardSelectedBorderStyle.Render(left + strings.Repeat("─", innerWidth) + right)
	}
	return border(left + strings.Repeat("─", innerWidth) + right)
}

func border(value string) string {
	return ui.DashboardBorderStyle.Render(value)
}

func outerBorder(value string, selected bool) string {
	if selected {
		return ui.DashboardSelectedBorderStyle.Render(value)
	}
	return border(value)
}

func gridBorder(value string, selected bool) string {
	if selected {
		return ui.DashboardSelectedBorderStyle.Render(value)
	}
	return border(value)
}

func slotsForLocalDay(forecast surf.Forecast, now time.Time) []surf.ForecastSlot {
	return slotsForLocalDayOffset(forecast, now, 0)
}

func slotsForLocalDayOffset(forecast surf.Forecast, now time.Time, dayOffset int) []surf.ForecastSlot {
	localNow := now.UTC().Add(forecast.UTCOffset)
	year, month, day := localNow.Date()
	selectedDate := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).AddDate(0, 0, dayOffset)
	year, month, day = selectedDate.Date()
	nextMidnight := selectedDate.AddDate(0, 0, 1)

	slots := make([]surf.ForecastSlot, 0, len(forecast.Slots))
	for _, slot := range forecast.Slots {
		localSlot := slot.Timestamp.UTC().Add(forecast.UTCOffset)
		sy, sm, sd := localSlot.Date()
		if sy == year && sm == month && sd == day || localSlot.Equal(nextMidnight) {
			slots = append(slots, slot)
		}
	}
	return slots
}

func slotsByHour(forecast surf.Forecast, now time.Time) map[int]surf.ForecastSlot {
	return slotsByHourForDay(forecast, now, 0)
}

func slotsByHourForDay(forecast surf.Forecast, now time.Time, dayOffset int) map[int]surf.ForecastSlot {
	slots := make(map[int]surf.ForecastSlot, len(dashboardHours))
	localNow := now.UTC().Add(forecast.UTCOffset)
	year, month, day := localNow.Date()
	selectedDate := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).AddDate(0, 0, dayOffset)
	year, month, day = selectedDate.Date()
	for _, slot := range slotsForLocalDayOffset(forecast, now, dayOffset) {
		localSlot := slot.Timestamp.UTC().Add(forecast.UTCOffset)
		sy, sm, sd := localSlot.Date()
		hour := localSlot.Hour()
		if sy != year || sm != month || sd != day {
			hour = 24
		}
		slots[hour] = slot
	}
	return slots
}

func tableCell(value string, width int) string {
	return ui.DashboardSlotStyle.Width(width).MaxWidth(width).Align(lipgloss.Center).Render(value)
}

func compactRating(rating string, width int) string {
	switch rating {
	case "Very Poor":
		rating = "VP"
	case "Poor to Fair":
		rating = "P–F"
	case "Fair to Good":
		rating = "F–G"
	case "Very Good":
		rating = "VG"
	}
	if width >= len([]rune(rating)) {
		return rating
	}
	if rating == "Poor" {
		return "P"
	}
	if rating == "Fair" {
		return "F"
	}
	if rating == "Good" {
		return "G"
	}
	if rating == "Epic" {
		return "E"
	}
	return rating
}

func isCurrentDashboardHour(hour int, now time.Time) bool {
	return hour < 24 && now.Hour()/3 == hour/3
}

func formatHour(value time.Time) string {
	hour := value.Hour()
	if hour == 0 {
		return "12am"
	}
	if hour == 12 {
		return "12pm"
	}
	if hour > 12 {
		return strconv.Itoa(hour-12) + "pm"
	}
	return strconv.Itoa(hour) + "am"
}

func formatDashboardHour(hour int) string {
	if hour == 24 {
		return "12a"
	}
	return strings.TrimSuffix(formatHour(time.Date(2000, time.January, 1, hour, 0, 0, 0, time.UTC)), "m")
}

func formatDashboardDate(value time.Time) string {
	return fmt.Sprintf("%d/%d", value.Month(), value.Day())
}

func formatHeight(height surf.SurfHeight) string {
	minHeight := strconv.FormatFloat(math.Round(height.Min), 'f', 0, 64)
	maxHeight := strconv.FormatFloat(math.Round(height.Max), 'f', 0, 64)
	plus := ""
	if height.Plus {
		plus = "+"
	}
	return fmt.Sprintf("%s–%s′%s", minHeight, maxHeight, plus)
}

func compactHeight(height surf.SurfHeight, width int) string {
	value := formatHeight(height)
	if len([]rune(value)) <= width {
		return value
	}
	value = strings.ReplaceAll(value, "′", "")
	if len([]rune(value)) <= width {
		return value
	}
	return strings.TrimSuffix(value, "+")
}
