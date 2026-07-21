package search

import (
	"fmt"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case SearchResultsMsg:
		if msg.RequestID != m.activeRequestID {
			return m, nil
		}
		m.Loading = false
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
		key := msg.Key()
		if m.InResults() {
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

		switch msg.String() {
		case "enter":
			return m, m.submit()
		case "backspace", "ctrl+h":
			if m.Query != "" && !m.Loading {
				_, size := utf8.DecodeLastRuneInString(m.Query)
				m.Query = m.Query[:len(m.Query)-size]
				m.Results = nil
				m.Cursor = 0
				m.HasSearched = false
				m.Err = nil
			}
		default:
			if key.Text != "" && !m.Loading {
				m.Query += key.Text
				m.Results = nil
				m.Cursor = 0
				m.HasSearched = false
				m.Err = nil
				m.Status = ""
			}
		}
	}

	return m, nil
}
