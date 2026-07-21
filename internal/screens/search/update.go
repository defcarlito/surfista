package search

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width)
		return m, nil

	case liveSearchMsg:
		query := strings.TrimSpace(m.Input.Value())
		if msg.RequestID != m.activeRequestID || msg.Query != query || m.mode != typingMode {
			return m, nil
		}
		return m, m.startSearch(msg.Query, msg.RequestID)

	case spinner.TickMsg:
		if !m.Loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd

	case SearchResultsMsg:
		if msg.RequestID != m.activeRequestID {
			return m, nil
		}
		m.Loading = false
		m.Pending = false
		m.HasSearched = true
		m.Err = nil
		m.Results = msg.Spots
		m.Cursor = 0
		return m, nil

	case SearchErrorMsg:
		if msg.RequestID != m.activeRequestID {
			return m, nil
		}
		m.Loading = false
		m.Pending = false
		m.HasSearched = true
		m.Results = nil
		m.Err = msg.Err
		return m, nil

	case SpotAddedMsg:
		m.statusID++
		statusID := m.statusID
		if msg.Err != nil {
			m.Err = fmt.Errorf("save tracked location: %w", msg.Err)
			m.Status = ""
			return m, nil
		}
		m.Err = nil
		if msg.Added {
			m.Status = fmt.Sprintf("Added %s to tracked locations.", msg.Spot.Name)
		} else {
			m.Status = fmt.Sprintf("%s is already tracked.", msg.Spot.Name)
		}
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return clearStatusMsg{StatusID: statusID}
		})

	case clearStatusMsg:
		if msg.StatusID == m.statusID {
			m.Status = ""
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.Selecting() {
			switch msg.String() {
			case "up", "k":
				if m.Cursor > 0 {
					m.Cursor--
				}
			case "down", "j":
				if m.Cursor < len(m.Results)-1 {
					m.Cursor++
				}
			case "enter":
				return m, m.addSelectedCmd()
			}
			return m, nil
		}

		if msg.String() == "enter" {
			if m.InResults() {
				m.mode = selectingMode
				m.Cursor = 0
				m.Input.Blur()
				return m, nil
			}
			return m, m.searchImmediately()
		}
	}

	if m.mode == selectingMode {
		return m, nil
	}

	previousValue := m.Input.Value()
	var inputCmd tea.Cmd
	m.Input, inputCmd = m.Input.Update(msg)
	if m.Input.Value() == previousValue {
		return m, inputCmd
	}

	liveCmd := m.queueLiveSearch()
	return m, tea.Batch(inputCmd, liveCmd)
}
