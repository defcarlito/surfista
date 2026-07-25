package dashboard

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"surfista/internal/surf"
)

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height
		m.ensureSelectedVisible()
		m.clampDetailsScroll()
	case ForecastLoadedMsg:
		state, tracked := m.forecasts[msg.SpotID]
		if !tracked {
			return m, nil
		}
		state.loading = false
		state.err = msg.Err
		if msg.Err == nil {
			state.forecast = msg.Forecast
			state.updatedAt = m.now()
			state.fetched = true
		}
		m.forecasts[msg.SpotID] = state
		if msg.Err == nil {
			m.saveForecastCache(msg.SpotID)
		}
		m.clampForecastDayOffset()
		m.applySort()
		m.finishSpotFetch(msg.SpotID)
	case ForecastDetailsLoadedMsg:
		state, tracked := m.details[msg.SpotID]
		if !tracked {
			return m, nil
		}
		state.loading = false
		state.err = msg.Err
		if msg.Err == nil {
			state.details = msg.Details
			state.updatedAt = m.now()
		}
		m.details[msg.SpotID] = state
		if msg.Err == nil {
			m.saveForecastCache(msg.SpotID)
			if m.sortMode == SortConditionHighToLow && m.forecastDayOffset > 0 {
				m.applySort()
			}
		}
		m.finishSpotFetch(msg.SpotID)
	case spinner.TickMsg:
		if !m.hasActiveFetches() {
			return m, nil
		}
		var cmd tea.Cmd
		m.refreshSpinner, cmd = m.refreshSpinner.Update(msg)
		return m, cmd
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
		if m.confirmRefresh {
			switch msg.String() {
			case "enter":
				m.confirmRefresh = false
				return m, m.refresh()
			case "esc":
				m.confirmRefresh = false
			}
			return m, nil
		}
		if m.detailsOpen {
			switch msg.String() {
			case "h", "left":
				previousDay := m.forecastDayOffset
				m.moveForecastDay(-1)
				if m.forecastDayOffset != previousDay {
					m.clampDetailsScroll()
				}
			case "l", "right":
				previousDay := m.forecastDayOffset
				m.moveForecastDay(1)
				if m.forecastDayOffset != previousDay {
					m.clampDetailsScroll()
				}
			case "j", "down":
				rows := m.detailsForecastRows()
				visible := m.detailsVisibleRowCount(len(rows))
				if m.detailsScroll < max(0, len(rows)-visible) {
					m.detailsScroll++
				}
			case "k", "up":
				if m.detailsScroll > 0 {
					m.detailsScroll--
				}
			case "u":
				return m, m.openDetailsURLCmd()
			case "esc":
				m.detailsOpen = false
				m.detailsSpot = surf.Spot{}
				m.detailsScroll = 0
			}
			return m, nil
		}
		switch msg.String() {
		case "h", "left":
			m.moveForecastDay(-1)
		case "l", "right":
			m.moveForecastDay(1)
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
		case "enter":
			return m, m.openSelectedDetails()
		case "s":
			return m, m.cycleSort()
		case "r":
			if m.canRefresh() {
				m.confirmRefresh = true
			}
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
