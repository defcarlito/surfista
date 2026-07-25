package app

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"surfista/internal/screens/dashboard"
	"surfista/internal/screens/search"
)

func (m Model) Init() tea.Cmd {
	if m.current != loadingScreen {
		return m.dashboard.Init()
	}
	return tea.Batch(m.dashboard.Init(), m.loading.Init(), m.startupWaitCmd())
}

func (m Model) startupWaitCmd() tea.Cmd {
	if m.current != loadingScreen {
		return nil
	}
	return tea.Tick(startupWaitLimit, func(time.Time) tea.Msg {
		return startupWaitExpiredMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.WindowSizeMsg); ok {
		var searchCmd, dashboardCmd, loadingCmd tea.Cmd
		m.search, searchCmd = m.search.Update(msg)
		m.dashboard, dashboardCmd = m.dashboard.Update(msg)
		m.loading, loadingCmd = m.loading.Update(msg)
		return m, tea.Batch(searchCmd, dashboardCmd, loadingCmd)
	}

	if _, ok := msg.(startupWaitExpiredMsg); ok {
		if m.current == loadingScreen {
			m.current = homeScreen
			return m, m.dashboard.FetchSpinnerTick()
		}
		return m, nil
	}

	switch msg.(type) {
	case dashboard.ForecastLoadedMsg, dashboard.ForecastDetailsLoadedMsg:
		m.dashboard, _ = m.dashboard.Update(msg)
		m.loading.SetCanSkip(m.dashboard.HasUsableForecasts())
		if m.current == loadingScreen {
			pending := m.dashboard.PendingInitialFetches()
			locationsLoaded, forecastsLoaded := m.dashboard.InitialFetchProgress()
			m.loading.SetProgress(locationsLoaded, forecastsLoaded)
			if pending == 0 {
				m.current = homeScreen
			}
		}
		return m, nil
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		if key.String() == "ctrl+c" {
			return m, tea.Quit
		}

		switch m.current {
		case homeScreen:
			if m.dashboard.ConfirmingRemoval() || m.dashboard.ConfirmingRefresh() || m.dashboard.ShowingDetails() {
				break
			}
			switch key.String() {
			case "/":
				if m.dashboard.HasSelection() {
					break
				}
				m.current = searchScreen
				return m, m.search.Focus()
			case "q":
				return m, tea.Quit
			}
		case searchScreen:
			if key.String() == "esc" {
				returnHome, cmd := m.search.Escape()
				if returnHome {
					m.current = homeScreen
				}
				return m, cmd
			}
		case loadingScreen:
			if key.String() == "enter" && m.loading.CanSkip() {
				m.current = homeScreen
				return m, m.dashboard.FetchSpinnerTick()
			}
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
	if m.current == loadingScreen {
		var cmd tea.Cmd
		m.loading, cmd = m.loading.Update(msg)
		return m, cmd
	}
	if m.current == homeScreen {
		var cmd tea.Cmd
		m.dashboard, cmd = m.dashboard.Update(msg)
		return m, cmd
	}

	return m, nil
}
