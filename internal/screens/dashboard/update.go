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
	}
	return m, nil
}
