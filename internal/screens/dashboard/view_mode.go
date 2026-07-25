package dashboard

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"surfista/internal/surf"
)

type dashboardViewMode uint8

const (
	dashboardViewSurf dashboardViewMode = iota
	dashboardViewWind
	dashboardViewSwell
	dashboardViewCount
)

func (m dashboardViewMode) label() string {
	switch m {
	case dashboardViewWind:
		return "wind"
	case dashboardViewSwell:
		return "swell"
	default:
		return "surf"
	}
}

func (m *Model) cycleDashboardView() tea.Cmd {
	m.viewMode = (m.viewMode + 1) % dashboardViewCount
	if m.viewMode == dashboardViewWind {
		return m.queueMissingWindDetails()
	}
	return nil
}

func (m Model) dashboardMetricValue(spotID string, slot surf.ForecastSlot, width int) string {
	switch m.viewMode {
	case dashboardViewWind:
		state := m.details[spotID]
		if detail, ok := forecastDetailSlotAt(state.details, slot.Timestamp); ok {
			return compactDashboardWind(detail.Wind, width)
		}
		if state.loading || state.queued {
			return m.refreshSpinner.View()
		}
		return "—"
	case dashboardViewSwell:
		if len(slot.Swells) == 0 {
			return "—"
		}
		return compactDashboardSwell(slot.Swells[0], width)
	default:
		return compactHeight(slot.SurfHeight, width)
	}
}

func forecastDetailSlotAt(details surf.ForecastDetails, timestamp time.Time) (surf.ForecastDetailSlot, bool) {
	for _, slot := range details.Slots {
		if slot.Timestamp.Equal(timestamp) {
			return slot, true
		}
	}
	return surf.ForecastDetailSlot{}, false
}

func compactDashboardWind(wind surf.Wind, width int) string {
	short, narrow := dashboardWindDirection(wind)
	speed := strconv.FormatFloat(math.Round(wind.Speed), 'f', 0, 64)
	return firstDashboardMetricThatFits(width,
		fmt.Sprintf("%s %s", short, speed),
		fmt.Sprintf("%s %s", narrow, speed),
		narrow+speed,
		short,
		narrow,
	)
}

func dashboardWindDirection(wind surf.Wind) (string, string) {
	directionType := strings.ToLower(strings.TrimSpace(wind.DirectionType))
	switch {
	case strings.Contains(directionType, "off"):
		return "off", "off"
	case strings.Contains(directionType, "cross"):
		return "cross", "x"
	case strings.Contains(directionType, "on"):
		return "on", "on"
	default:
		direction := compassDirection(wind.Direction)
		return direction, direction
	}
}

func compactDashboardWindUnit(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "kt", "kts", "knot", "knots":
		return "kt"
	case "km/h", "kmh", "kph":
		return "kph"
	default:
		return strings.ToLower(strings.TrimSpace(unit))
	}
}

func (m Model) dashboardWindUnit() string {
	for _, spot := range m.spots {
		if unit := strings.TrimSpace(m.details[spot.ID].details.Units.WindSpeed); unit != "" {
			return compactDashboardWindUnit(unit)
		}
	}
	return compactDashboardWindUnit("")
}

func compactDashboardSwell(swell surf.Swell, width int) string {
	height := formatDetailNumber(swell.Height) + "′"
	roundedHeight := strconv.FormatFloat(math.Round(swell.Height), 'f', 0, 64) + "′"
	period := strconv.FormatFloat(math.Round(swell.Period), 'f', 0, 64) + "s"
	direction := compassDirection(swell.Direction)
	return firstDashboardMetricThatFits(width,
		strings.Join([]string{height, period, direction}, " "),
		strings.Join([]string{roundedHeight, period, direction}, " "),
		period+" "+direction,
		height+" "+period,
		period,
		direction,
	)
}

func firstDashboardMetricThatFits(width int, candidates ...string) string {
	for _, candidate := range candidates {
		if ansi.StringWidth(candidate) <= width {
			return candidate
		}
	}
	if len(candidates) == 0 || width <= 0 {
		return ""
	}
	return ansi.Truncate(candidates[len(candidates)-1], width, "")
}
