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
		if m.canSkip {
			help := ui.LoadingHelpStyle.Width(contentWidth).Render("enter use cache while updates continue")
			return lipgloss.JoinVertical(lipgloss.Center, content, "", help)
		}
		return content
	}
	bodyHeight := m.terminalHeight
	if m.canSkip {
		bodyHeight--
	}
	body := lipgloss.Place(
		m.terminalWidth,
		max(0, bodyHeight),
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
	if !m.canSkip {
		return body
	}
	help := ui.LoadingHelpStyle.Width(m.terminalWidth).Render("enter use cache while updates continue")
	return lipgloss.JoinVertical(lipgloss.Left, body, help)
}
