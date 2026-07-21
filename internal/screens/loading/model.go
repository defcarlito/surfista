package loading

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"surfista/internal/ui"
)

// Model presents progress while the dashboard fetches the initial forecasts.
type Model struct {
	Spinner spinner.Model

	total          int
	completed      int
	terminalWidth  int
	terminalHeight int
}

func New(total int) Model {
	loader := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(ui.SearchSpinnerStyle),
	)
	return Model{Spinner: loader, total: max(0, total)}
}

func (m Model) Init() tea.Cmd {
	if m.total == 0 {
		return nil
	}
	return m.Spinner.Tick
}

func (m *Model) SetCompleted(completed int) {
	m.completed = max(0, min(completed, m.total))
}
