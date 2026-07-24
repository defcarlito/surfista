package app

import (
	tea "charm.land/bubbletea/v2"

	"surfista/internal/ui"
)

func (m Model) View() tea.View {
	content := m.homeView()
	switch m.current {
	case loadingScreen:
		content = m.loading.View()
	case searchScreen:
		content = m.search.View()
	}

	view := tea.NewView(content)
	view.AltScreen = true
	view.BackgroundColor = ui.AppBackgroundColor
	view.ForegroundColor = ui.OceanPalette.White
	view.WindowTitle = "Surfista"
	return view
}

func (m Model) homeView() string {
	return m.dashboard.View()
}
