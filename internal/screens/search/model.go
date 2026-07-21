package search

import (
	"context"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"surfista/internal/surf"
	"surfista/internal/ui"
)

const (
	requestTimeout          = 20 * time.Second
	maxContentWidth         = 60
	wideHorizontalMargin    = 2
	narrowHorizontalMargin  = 1
	narrowTerminalThreshold = 20
)

type Tracker interface {
	Add(surf.Spot) (bool, error)
}

type Model struct {
	Results     []surf.Spot
	Cursor      int
	Loading     bool
	HasSearched bool
	Err         error
	Status      string
	Input       textinput.Model
	Spinner     spinner.Model

	searcher        surf.SpotSearcher
	tracker         Tracker
	nextRequestID   uint64
	activeRequestID uint64
	statusID        uint64
	terminalWidth   int
	contentWidth    int
}

func New(searcher surf.SpotSearcher, tracker Tracker) Model {
	input := textinput.New()
	input.Prompt = ""
	input.Placeholder = "Search for a surf spot or location"
	input.CharLimit = 120
	input.SetWidth(maxContentWidth - ui.SearchInputFrameWidth)
	styles := input.Styles()
	styles.Focused.Text = ui.SearchTextStyle
	styles.Focused.Placeholder = ui.SearchPlaceholderStyle
	styles.Blurred.Text = ui.SearchTextStyle
	styles.Blurred.Placeholder = ui.SearchPlaceholderStyle
	input.SetStyles(styles)
	_ = input.Focus()

	loader := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(ui.SearchSpinnerStyle),
	)

	return Model{
		searcher:     searcher,
		tracker:      tracker,
		Input:        input,
		Spinner:      loader,
		contentWidth: maxContentWidth,
	}
}

func (m Model) InResults() bool {
	return len(m.Results) > 0 && !m.Loading
}

func (m *Model) Focus() tea.Cmd {
	return m.Input.Focus()
}

func (m Model) ContentWidth() int {
	return m.contentWidth
}

func (m *Model) resize(width int) {
	m.terminalWidth = width
	m.contentWidth = responsiveContentWidth(width)
	m.Input.SetWidth(max(1, m.contentWidth-ui.SearchInputFrameWidth))
}

func responsiveContentWidth(terminalWidth int) int {
	if terminalWidth <= 0 {
		return maxContentWidth
	}
	margin := wideHorizontalMargin
	if terminalWidth < narrowTerminalThreshold {
		margin = narrowHorizontalMargin
	}
	return max(1, min(maxContentWidth, terminalWidth-(margin*2)))
}

func (m *Model) Escape() bool {
	if m.Input.Value() == "" && len(m.Results) == 0 && !m.Loading && m.Err == nil && m.Status == "" {
		return true
	}

	m.Input.Reset()
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
	query := strings.TrimSpace(m.Input.Value())
	if query == "" || m.Loading || m.searcher == nil {
		return nil
	}

	m.Input.SetValue(query)
	m.Loading = true
	m.HasSearched = true
	m.Err = nil
	m.Status = ""
	m.Results = nil
	m.Cursor = 0
	m.nextRequestID++
	m.activeRequestID = m.nextRequestID
	return tea.Batch(m.searchCmd(query, m.activeRequestID), m.Spinner.Tick)
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
