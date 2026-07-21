package dashboard

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"surfista/internal/surf"
	"surfista/internal/ui"
)

const (
	maxDashboardWidth = 114
	pageMargin        = 2
	maxSlotWidth      = 10
	gridBorderWidth   = 10
)

var dashboardHours = [...]int{0, 3, 6, 9, 12, 15, 18, 21, 24}

func (m Model) View() string {
	width := m.contentWidth()
	sections := []string{
		ui.DashboardTitleStyle.Width(width).Render(ui.Title("Surfista")),
		ui.DashboardSubtitleStyle.Width(width).Render("Today's surf conditions"),
	}

	if m.loadErr != nil {
		sections = append(sections, "", ui.ErrorStyle.Width(width).Render("Could not load favorite locations: "+m.loadErr.Error()))
	}
	if len(m.spots) == 0 {
		sections = append(sections, "", ui.DashboardEmptyStyle.Width(width).Render("No favorite surf spots yet."))
	} else {
		sections = append(sections, "", m.forecastTable(width))
	}

	sections = append(sections, "", ui.DashboardHelpStyle.Width(width).Render("s or / search Surfline • q quit"))
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	if m.terminalWidth <= 0 {
		return content
	}
	return lipgloss.PlaceHorizontal(m.terminalWidth, lipgloss.Center, content)
}

func (m Model) contentWidth() int {
	if m.terminalWidth <= 0 {
		return maxDashboardWidth
	}
	return max(1, min(maxDashboardWidth, m.terminalWidth-(pageMargin*2)))
}

func (m Model) forecastTable(width int) string {
	slotWidth := max(1, min(maxSlotWidth, (width-gridBorderWidth)/len(dashboardHours)))
	cardWidth := slotWidth*len(dashboardHours) + gridBorderWidth

	headerCells := make([]string, 0, len(dashboardHours))
	for _, hour := range dashboardHours {
		headerCells = append(headerCells, tableCell(formatDashboardHour(hour), slotWidth, false))
	}
	rows := []string{" " + strings.Join(headerCells, " ") + " "}

	for _, spot := range m.spots {
		rows = append(rows, "", m.spotCard(spot, slotWidth))
	}
	table := strings.Join(rows, "\n")
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(
		lipgloss.NewStyle().Width(cardWidth).Render(table),
	)
}

func (m Model) spotCard(spot surf.Spot, slotWidth int) string {
	state := m.forecasts[spot.ID]
	innerWidth := slotWidth*len(dashboardHours) + len(dashboardHours) - 1
	name := ui.DashboardSpotStyle.Width(innerWidth).MaxWidth(innerWidth).Align(lipgloss.Center).Render(spot.Name)

	if state.loading {
		return statusCard(name, ui.Muted("Loading today's forecast…"), innerWidth)
	}
	if state.err != nil {
		return statusCard(name, ui.Error("Forecast unavailable: "+state.err.Error()), innerWidth)
	}

	now := m.now()
	slots := slotsByHour(state.forecast, now)
	if len(slots) == 0 {
		return statusCard(name, ui.Muted("No forecast available for today."), innerWidth)
	}

	ratings := make([]string, 0, len(dashboardHours))
	heights := make([]string, 0, len(dashboardHours))
	for _, hour := range dashboardHours {
		slot, ok := slots[hour]
		if !ok {
			ratings = append(ratings, tableCell("—", slotWidth, false))
			heights = append(heights, tableCell("—", slotWidth, false))
			continue
		}
		current := isCurrentDashboardHour(hour, now)
		ratings = append(ratings, tableCell(compactRating(slot.Rating, slotWidth), slotWidth, current))
		heights = append(heights, tableCell(compactHeight(slot.SurfHeight, slotWidth), slotWidth, current))
	}

	return strings.Join([]string{
		solidBorder("╭", "╮", innerWidth),
		borderedLine(name),
		segmentedBorder("├", "┬", "┤", slotWidth),
		segmentedLine(ratings),
		segmentedLine(heights),
		segmentedBorder("╰", "┴", "╯", slotWidth),
	}, "\n")
}

func statusCard(name, status string, innerWidth int) string {
	centeredStatus := lipgloss.NewStyle().Width(innerWidth).MaxWidth(innerWidth).Align(lipgloss.Center).Render(status)
	return strings.Join([]string{
		solidBorder("╭", "╮", innerWidth),
		borderedLine(name),
		solidBorder("├", "┤", innerWidth),
		borderedLine(centeredStatus),
		solidBorder("╰", "╯", innerWidth),
	}, "\n")
}

func borderedLine(content string) string {
	return border("│") + content + border("│")
}

func segmentedLine(cells []string) string {
	return border("│") + strings.Join(cells, border("│")) + border("│")
}

func segmentedBorder(left, divider, right string, slotWidth int) string {
	segments := make([]string, len(dashboardHours))
	for index := range segments {
		segments[index] = strings.Repeat("─", slotWidth)
	}
	return border(left + strings.Join(segments, divider) + right)
}

func solidBorder(left, right string, innerWidth int) string {
	return border(left + strings.Repeat("─", innerWidth) + right)
}

func border(value string) string {
	return ui.DashboardBorderStyle.Render(value)
}

func slotsForLocalDay(forecast surf.Forecast, now time.Time) []surf.ForecastSlot {
	localNow := now.UTC().Add(forecast.UTCOffset)
	year, month, day := localNow.Date()
	nextMidnight := time.Date(year, month, day+1, 0, 0, 0, 0, time.UTC)

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
	slots := make(map[int]surf.ForecastSlot, len(dashboardHours))
	localNow := now.UTC().Add(forecast.UTCOffset)
	year, month, day := localNow.Date()
	for _, slot := range slotsForLocalDay(forecast, now) {
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

func tableCell(value string, width int, current bool) string {
	style := ui.DashboardSlotStyle.Width(width).MaxWidth(width).Align(lipgloss.Center)
	if current {
		style = ui.DashboardCurrentSlotStyle.Width(width).MaxWidth(width).Align(lipgloss.Center)
	}
	return style.Render(value)
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
