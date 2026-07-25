package loading

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"surfista/internal/ui"
)

// Model presents progress while the dashboard fetches the initial forecasts.
type Model struct {
	Spinner spinner.Model

	locations       int
	forecastsLoaded int
	canSkip         bool
	terminalWidth   int
	terminalHeight  int
}

func New(locations int, canSkip bool) Model {
	loader := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(ui.SearchSpinnerStyle),
	)
	return Model{
		Spinner:   loader,
		locations: max(0, locations),
		canSkip:   canSkip,
	}
}

func (m Model) Init() tea.Cmd {
	if m.locations == 0 {
		return nil
	}
	return m.Spinner.Tick
}

func (m *Model) SetProgress(forecastsLoaded int) {
	m.forecastsLoaded = max(0, min(forecastsLoaded, m.locations))
}

func (m Model) CanSkip() bool {
	return m.canSkip
}

func (m *Model) SetCanSkip(canSkip bool) {
	m.canSkip = canSkip
}
