package search

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"surfista/internal/surf"
)

const requestTimeout = 20 * time.Second

type Tracker interface {
	Add(surf.Spot) (bool, error)
}

type Model struct {
	Query       string
	Results     []surf.Spot
	Cursor      int
	Loading     bool
	HasSearched bool
	Err         error
	Status      string

	searcher        surf.SpotSearcher
	tracker         Tracker
	nextRequestID   uint64
	activeRequestID uint64
	statusID        uint64
}

func New(searcher surf.SpotSearcher, tracker Tracker) Model {
	return Model{searcher: searcher, tracker: tracker}
}

func (m Model) InResults() bool {
	return len(m.Results) > 0 && !m.Loading
}

func (m *Model) Escape() bool {
	if m.Query == "" && len(m.Results) == 0 && !m.Loading && m.Err == nil && m.Status == "" {
		return true
	}

	m.Query = ""
	m.Results = nil
	m.Cursor = 0
	m.Loading = false
	m.HasSearched = false
	m.Err = nil
	m.Status = ""
	m.activeRequestID++ // Any in-flight response is now stale.
	return false
}

func (m Model) searchCmd(query string, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()

		spots, err := m.searcher.SearchSpots(ctx, query)
		if err != nil {
			return SearchErrorMsg{RequestID: requestID, Query: query, Err: err}
		}
		return SearchResultsMsg{RequestID: requestID, Query: query, Spots: spots}
	}
}

func (m *Model) submit() tea.Cmd {
	query := strings.TrimSpace(m.Query)
	if query == "" || m.Loading || m.searcher == nil {
		return nil
	}

	m.Query = query
	m.Loading = true
	m.HasSearched = true
	m.Err = nil
	m.Status = ""
	m.Results = nil
	m.Cursor = 0
	m.nextRequestID++
	m.activeRequestID = m.nextRequestID
	return m.searchCmd(query, m.activeRequestID)
}

func (m Model) addSelectedCmd() tea.Cmd {
	if len(m.Results) == 0 || m.Cursor < 0 || m.Cursor >= len(m.Results) || m.tracker == nil {
		return nil
	}
	spot := m.Results[m.Cursor]
	return func() tea.Msg {
		added, err := m.tracker.Add(spot)
		return SpotAddedMsg{Spot: spot, Added: added, Err: err}
	}
}
