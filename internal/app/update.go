package app

import (
	tea "charm.land/bubbletea/v2"

	"surfista/internal/screens/search"
)

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return m, cmd
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
		m.tracked = append(m.tracked, added.Spot)
	}

	if m.current == searchScreen {
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return m, cmd
	}

	return m, nil
}
