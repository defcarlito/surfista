package dashboard

import (
	"time"

	"surfista/internal/surf"
)

func (m Model) maxForecastDayOffset() int {
	maximum := 0
	now := m.now()
	for _, state := range m.forecasts {
		forecast := state.forecast
		localToday := localDate(now, forecast.UTCOffset)
		for _, slot := range forecast.Slots {
			localSlot := slot.Timestamp.UTC().Add(forecast.UTCOffset)
			// A lone midnight can be the closing boundary for the previous day,
			// not evidence that a full additional forecast day is available.
			if localSlot.Hour() == 0 {
				continue
			}
			offset := calendarDayOffset(localToday, localDate(slot.Timestamp, forecast.UTCOffset))
			if offset > maximum {
				maximum = offset
			}
		}
	}
	return maximum
}

func (m *Model) moveForecastDay(delta int) {
	maximum := m.maxForecastDayOffset()
	next := min(max(0, m.forecastDayOffset+delta), maximum)
	if next == m.forecastDayOffset {
		return
	}
	m.forecastDayOffset = next
	if m.sortMode == SortConditionHighToLow {
		m.applySort()
	}
}

func (m *Model) clampForecastDayOffset() {
	m.forecastDayOffset = min(max(0, m.forecastDayOffset), m.maxForecastDayOffset())
}

func (m Model) forecastHeaderSpotID() string {
	if m.detailsOpen && m.detailsSpot.ID != "" {
		return m.detailsSpot.ID
	}
	if m.HasSelection() {
		return m.spots[m.selectedIndex].ID
	}
	return ""
}

func (m Model) dashboardForecastDate(dayOffset int) time.Time {
	if spotID := m.forecastHeaderSpotID(); spotID != "" {
		forecast := m.forecasts[spotID].forecast
		if len(forecast.Slots) > 0 {
			return localDate(m.now(), forecast.UTCOffset).AddDate(0, 0, dayOffset)
		}
	}
	for _, spot := range m.spots {
		forecast := m.forecasts[spot.ID].forecast
		if len(forecast.Slots) > 0 {
			return localDate(m.now(), forecast.UTCOffset).AddDate(0, 0, dayOffset)
		}
	}
	now := m.now()
	year, month, day := now.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC).AddDate(0, 0, dayOffset)
}

func localDate(value time.Time, utcOffset time.Duration) time.Time {
	local := value.UTC().Add(utcOffset)
	year, month, day := local.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func calendarDayOffset(start, end time.Time) int {
	return int(end.Sub(start) / (24 * time.Hour))
}

func sunlightForLocalDay(details surf.ForecastDetails, now time.Time, utcOffset time.Duration, dayOffset int) (surf.SunlightDay, bool) {
	targetDate := localDate(now, utcOffset).AddDate(0, 0, dayOffset)
	for _, day := range details.Sunlight {
		reference := firstSunlightTime(day)
		if !reference.IsZero() && localDate(reference, utcOffset).Equal(targetDate) {
			return day, true
		}
	}
	return surf.SunlightDay{}, false
}

func firstSunlightTime(day surf.SunlightDay) time.Time {
	for _, candidate := range []time.Time{day.Sunrise, day.Sunset, day.Dawn, day.Dusk} {
		if !candidate.IsZero() {
			return candidate
		}
	}
	return time.Time{}
}
