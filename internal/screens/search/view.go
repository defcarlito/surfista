package search

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"surfista/internal/ui"
)

func (m Model) View() string {
	width := max(1, m.contentWidth)
	input := ui.SearchInputStyle.Width(width).Render(m.Input.View())
	body := m.resultsView(width)
	help := m.helpView(width)

	sections := []string{input, "", body}
	if m.Status != "" {
		sections = append(sections, "", ui.SuccessStyle.Width(width).Render(m.Status))
	}
	sections = append(sections, "", help)
	controls := lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Left).
		Render(lipgloss.JoinVertical(lipgloss.Left, sections...))

	header := ui.GradientText(ui.SearchBanner)
	headerWidth := lipgloss.Width(ui.SearchBanner)
	if m.terminalWidth > 0 && headerWidth > m.terminalWidth-(wideHorizontalMargin*2) {
		header = ui.Title("surfista")
		headerWidth = lipgloss.Width(header)
	}
	layoutWidth := max(width, headerWidth)
	header = ui.SearchTitleStyle.Width(layoutWidth).Render(header)
	controls = lipgloss.PlaceHorizontal(layoutWidth, lipgloss.Center, controls)

	column := lipgloss.NewStyle().
		Width(layoutWidth).
		Align(lipgloss.Left).
		MarginTop(1).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, "", controls))

	if m.terminalWidth <= 0 {
		return column
	}
	return lipgloss.PlaceHorizontal(m.terminalWidth, lipgloss.Center, column)
}

func (m Model) resultsView(width int) string {
	switch {
	case m.Loading:
		return ui.SearchResultStyle.Width(width).Render(
			fmt.Sprintf("%s Searching Surfline for %q…", m.Spinner.View(), m.Input.Value()),
		)
	case m.Err != nil:
		return ui.ErrorStyle.Width(width).Render("Search failed: " + m.Err.Error())
	case !m.HasSearched:
		return ui.SearchEmptyStyle.Width(width).Render("Type a surf spot or location, then press Enter.")
	case len(m.Results) == 0:
		return ui.SearchEmptyStyle.Width(width).Render("No matching Surfline spots found.")
	}

	rows := make([]string, 0, len(m.Results)+1)
	for index, spot := range m.Results {
		marker := "  "
		selected := index == m.Cursor
		if selected {
			marker = "> "
		}

		location := strings.Trim(strings.Join([]string{spot.Region, spot.Country}, ", "), ", ")
		line := marker + spot.Name
		if location != "" {
			line += " — " + location
		}
		if selected {
			rows = append(rows, ui.GradientBackground(" "+line, width))
		} else {
			rows = append(rows, ui.SearchResultStyle.Width(width).MaxWidth(width).Render(line))
		}

		if selected && spot.URL != "" {
			rows = append(rows, ui.SearchURLStyle.MaxWidth(width).Render(spot.URL))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m Model) helpView(width int) string {
	text := "Enter search • Esc clear/back • Ctrl+C quit"
	if m.InResults() {
		text = "↑/k ↓/j navigate • Enter track • Esc clear"
	}
	return ui.SearchHelpStyle.Width(width).Render(text)
}
