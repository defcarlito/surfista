package dashboard

import tea "charm.land/bubbletea/v2"

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
	case ForecastLoadedMsg:
		if _, tracked := m.forecasts[msg.SpotID]; !tracked {
			return m, nil
		}
		m.forecasts[msg.SpotID] = forecastState{
			forecast: msg.Forecast,
			err:      msg.Err,
		}
	case tea.KeyPressMsg:
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
		}
	}
	return m, nil
}
