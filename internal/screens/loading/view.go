package loading

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"surfista/internal/ui"
)

func (m Model) View() string {
	banner := ui.GradientText(ui.LoadingBanner)
	status := fmt.Sprintf("%s Fetching favorite forecasts… %d/%d", m.Spinner.View(), m.completed, m.total)
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		banner,
		"",
		ui.LoadingStatusStyle.Width(lipgloss.Width(ui.LoadingBanner)).Render(status),
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
