package app

import (
	tea "charm.land/bubbletea/v2"

	"surfista/internal/screens/dashboard"
	"surfista/internal/screens/search"
)

func (m Model) Init() tea.Cmd {
	return m.dashboard.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		var searchCmd, dashboardCmd tea.Cmd
		m.search, searchCmd = m.search.Update(msg)
		m.dashboard, dashboardCmd = m.dashboard.Update(msg)
		return m, tea.Batch(searchCmd, dashboardCmd)
	}

	if _, ok := msg.(dashboard.ForecastLoadedMsg); ok {
		m.dashboard, _ = m.dashboard.Update(msg)
		return m, nil
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		if key.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.current == homeScreen {
			switch key.String() {
			case "s", "/":
				m.current = searchScreen
				return m, m.search.Focus()
			case "q":
				return m, tea.Quit
			}
		} else if key.String() == "esc" {
			returnHome, cmd := m.search.Escape()
			if returnHome {
				m.current = homeScreen
			}
			return m, cmd
		}
	}

	if added, ok := msg.(search.SpotAddedMsg); ok && added.Err == nil && added.Added {
		cmd := m.dashboard.Add(added.Spot)
		if m.current == searchScreen {
			var searchCmd tea.Cmd
			m.search, searchCmd = m.search.Update(msg)
			return m, tea.Batch(cmd, searchCmd)
		}
		return m, cmd
	}

	if m.current == searchScreen {
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return m, cmd
	}

	return m, nil
}
