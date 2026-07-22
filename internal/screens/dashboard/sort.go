package dashboard

import (
	"sort"

	tea "charm.land/bubbletea/v2"
)

type SortMode string

const (
	SortTimeAdded          SortMode = "time_added"
	SortConditionHighToLow SortMode = "condition_high_to_low"
)

type SortStore interface {
	LoadSortMode() (string, error)
	SaveSortMode(mode string) error
}

func parseSortMode(value string) (SortMode, bool) {
	mode := SortMode(value)
	switch mode {
	case SortTimeAdded, SortConditionHighToLow:
		return mode, true
	default:
		return "", false
	}
}

func (m SortMode) label() string {
	switch m {
	case SortConditionHighToLow:
		return "conditions"
	default:
		return "time added"
	}
}

func conditionRank(rating string) int {
	switch rating {
	case "Very Poor":
		return 0
	case "Poor":
		return 1
	case "Poor to Fair":
		return 2
	case "Fair":
		return 3
	case "Fair to Good":
		return 4
	case "Good":
		return 5
	case "Very Good":
		return 6
	case "Epic":
		return 7
	default:
		return -1
	}
}

func (m *Model) applySort() {
	selectedID := ""
	if m.HasSelection() {
		selectedID = m.spots[m.selectedIndex].ID
	}

	ranks := make(map[string]int, len(m.spots))
	if m.sortMode == SortConditionHighToLow {
		now := m.now()
		currentHour := now.Hour() / 3 * 3
		for _, spot := range m.spots {
			state := m.forecasts[spot.ID]
			rank := -1
			if !state.loading && state.err == nil {
				if slot, ok := slotsByHour(state.forecast, now)[currentHour]; ok {
					rank = conditionRank(slot.Rating)
				}
			}
			ranks[spot.ID] = rank
		}
	}

	sort.SliceStable(m.spots, func(i, j int) bool {
		left, right := m.spots[i], m.spots[j]
		if m.sortMode == SortConditionHighToLow && ranks[left.ID] != ranks[right.ID] {
			return ranks[left.ID] > ranks[right.ID]
		}
		return m.addedOrder[left.ID] < m.addedOrder[right.ID]
	})

	m.selectedIndex = -1
	if selectedID != "" {
		for index, spot := range m.spots {
			if spot.ID == selectedID {
				m.selectedIndex = index
				break
			}
		}
	}
	m.scrollOffset = 0
	m.ensureSelectedVisible()
}

func (m *Model) cycleSort() tea.Cmd {
	if m.sortMode == SortTimeAdded {
		m.sortMode = SortConditionHighToLow
	} else {
		m.sortMode = SortTimeAdded
	}
	m.applySort()

	if m.sortStore == nil {
		return nil
	}
	mode := m.sortMode
	return func() tea.Msg {
		return SortModeSavedMsg{Mode: mode, Err: m.sortStore.SaveSortMode(string(mode))}
	}
}
