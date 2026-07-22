package dashboard

import (
	"sort"
	"time"

	tea "charm.land/bubbletea/v2"

	"surfista/internal/surf"
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

type conditionSortValue struct {
	available bool
	score     float64
	rank      int
	maxHeight float64
	minHeight float64
}

type ratedForecastSlot struct {
	timestamp time.Time
	rank      int
}

func currentConditionSortValue(forecast surf.Forecast, now time.Time, dayOffset int) conditionSortValue {
	value := conditionSortValue{rank: -1}
	currentHour := now.Hour() / 3 * 3
	slot, ok := slotsByHourForDay(forecast, now, dayOffset)[currentHour]
	if !ok {
		return value
	}
	value.rank = conditionRank(slot.Rating)
	if value.rank < 0 {
		return value
	}
	value.available = true
	value.maxHeight = slot.SurfHeight.Max
	value.minHeight = slot.SurfHeight.Min
	return value
}

func futureConditionSortValue(forecast surf.Forecast, details surf.ForecastDetails, now time.Time, dayOffset int) conditionSortValue {
	value := conditionSortValue{rank: -1}
	day, ok := sunlightForLocalDay(details, now, forecast.UTCOffset, dayOffset)
	if !ok || day.Sunrise.IsZero() || day.Sunset.IsZero() || !day.Sunset.After(day.Sunrise) {
		return value
	}

	daylightSlots := make([]ratedForecastSlot, 0, len(forecast.Slots))
	var rankTotal, maxHeightTotal, minHeightTotal float64
	for _, slot := range forecast.Slots {
		if slot.Timestamp.Before(day.Sunrise) || slot.Timestamp.After(day.Sunset) {
			continue
		}
		rank := conditionRank(slot.Rating)
		if rank < 0 {
			continue
		}
		daylightSlots = append(daylightSlots, ratedForecastSlot{timestamp: slot.Timestamp, rank: rank})
		rankTotal += float64(rank)
		maxHeightTotal += slot.SurfHeight.Max
		minHeightTotal += slot.SurfHeight.Min
	}
	if len(daylightSlots) == 0 {
		return value
	}

	sort.Slice(daylightSlots, func(i, j int) bool {
		return daylightSlots[i].timestamp.Before(daylightSlots[j].timestamp)
	})
	daylightAverage := rankTotal / float64(len(daylightSlots))
	bestThreeHourAverage := daylightAverage
	foundThreeHourWindow := false
	for start := 0; start+2 < len(daylightSlots); start++ {
		first, second, third := daylightSlots[start], daylightSlots[start+1], daylightSlots[start+2]
		if second.timestamp.Sub(first.timestamp) != time.Hour || third.timestamp.Sub(second.timestamp) != time.Hour {
			continue
		}
		windowAverage := float64(first.rank+second.rank+third.rank) / 3
		if !foundThreeHourWindow || windowAverage > bestThreeHourAverage {
			bestThreeHourAverage = windowAverage
			foundThreeHourWindow = true
		}
	}

	value.available = true
	value.score = daylightAverage*0.75 + bestThreeHourAverage*0.25
	value.maxHeight = maxHeightTotal / float64(len(daylightSlots))
	value.minHeight = minHeightTotal / float64(len(daylightSlots))
	return value
}

func (m *Model) applySort() {
	selectedID := ""
	if m.HasSelection() {
		selectedID = m.spots[m.selectedIndex].ID
	}

	conditions := make(map[string]conditionSortValue, len(m.spots))
	if m.sortMode == SortConditionHighToLow {
		now := m.now()
		for _, spot := range m.spots {
			forecast := m.forecasts[spot.ID].forecast
			if m.forecastDayOffset == 0 {
				conditions[spot.ID] = currentConditionSortValue(forecast, now, m.forecastDayOffset)
			} else {
				conditions[spot.ID] = futureConditionSortValue(forecast, m.details[spot.ID].details, now, m.forecastDayOffset)
			}
		}
	}

	sort.SliceStable(m.spots, func(i, j int) bool {
		left, right := m.spots[i], m.spots[j]
		if m.sortMode == SortConditionHighToLow {
			leftCondition := conditions[left.ID]
			rightCondition := conditions[right.ID]
			if leftCondition.available != rightCondition.available {
				return leftCondition.available
			}
			if m.forecastDayOffset == 0 {
				if leftCondition.rank != rightCondition.rank {
					return leftCondition.rank > rightCondition.rank
				}
			} else if leftCondition.score != rightCondition.score {
				return leftCondition.score > rightCondition.score
			}
			if leftCondition.maxHeight != rightCondition.maxHeight {
				return leftCondition.maxHeight > rightCondition.maxHeight
			}
			if leftCondition.minHeight != rightCondition.minHeight {
				return leftCondition.minHeight > rightCondition.minHeight
			}
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
