package dashboard

import (
	tea "charm.land/bubbletea/v2"

	"surfista/internal/surf"
)

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height
		m.ensureSelectedVisible()
	case ForecastLoadedMsg:
		if _, tracked := m.forecasts[msg.SpotID]; !tracked {
			return m, nil
		}
		m.forecasts[msg.SpotID] = forecastState{
			forecast: msg.Forecast,
			err:      msg.Err,
		}
		m.applySort()
	case SpotRemovedMsg:
		if !m.confirmRemoval || msg.SpotID != m.removalSpot.ID {
			return m, nil
		}
		m.removing = false
		if msg.Err != nil {
			m.removalErr = msg.Err
			return m, nil
		}
		m.removeSpot(msg.SpotID)
	case tea.KeyPressMsg:
		if m.confirmRemoval {
			switch msg.String() {
			case "enter":
				if !m.removing {
					m.removing = true
					m.removalErr = nil
					return m, m.removeCmd(m.removalSpot.ID)
				}
			case "esc":
				if !m.removing {
					m.confirmRemoval = false
					m.removalSpot = surf.Spot{}
					m.removalErr = nil
				}
			}
			return m, nil
		}
		switch msg.String() {
		case "j", "down":
			if len(m.spots) == 0 {
				break
			}
			if m.selectedIndex < 0 {
				m.selectedIndex = 0
			} else if m.selectedIndex < len(m.spots)-1 {
				m.selectedIndex++
			}
		case "k", "up":
			if len(m.spots) == 0 {
				break
			}
			if m.selectedIndex < 0 {
				m.selectedIndex = len(m.spots) - 1
			} else if m.selectedIndex > 0 {
				m.selectedIndex--
			}
		case "esc":
			m.selectedIndex = -1
		case "s":
			return m, m.cycleSort()
		case "u":
			return m, m.openSelectedURLCmd()
		case "x":
			if m.selectedIndex >= 0 && m.selectedIndex < len(m.spots) {
				m.confirmRemoval = true
				m.removalSpot = m.spots[m.selectedIndex]
				m.removalErr = nil
			}
		}
		m.ensureSelectedVisible()
	}
	return m, nil
}
