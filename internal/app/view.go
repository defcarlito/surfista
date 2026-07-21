package app

import (
	tea "charm.land/bubbletea/v2"
)

func (m Model) View() tea.View {
	content := m.homeView()
	if m.current == searchScreen {
		content = m.search.View()
	}

	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "Surfista"
	return view
}

func (m Model) homeView() string {
	return m.dashboard.View()
}
