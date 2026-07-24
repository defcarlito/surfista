package loading

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"surfista/internal/ui"
)

func (m Model) View() string {
	banner := ui.GradientText(ui.LoadingBanner)
	status := ""
	if m.locationsLoaded < m.locations {
		status = fmt.Sprintf("%s Locations loaded %d/%d", m.Spinner.View(), m.locationsLoaded, m.locations)
	} else {
		status = fmt.Sprintf("%s Fetching forecasts %d/%d", m.Spinner.View(), m.forecastsLoaded, m.locations)
	}
	contentWidth := lipgloss.Width(ui.LoadingBanner)
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		banner,
		"",
		ui.LoadingStatusStyle.Width(contentWidth).Render(status),
	)

	if m.terminalWidth <= 0 || m.terminalHeight <= 0 {
		return content
	}
	return lipgloss.Place(
		m.terminalWidth,
		m.terminalHeight,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}
