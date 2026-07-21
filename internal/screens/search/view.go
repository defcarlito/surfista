package search

import (
	"fmt"
	"strings"

	"surfista/internal/ui"
)

func (m Model) View() string {
	var view strings.Builder
	view.WriteString(ui.Title("Search Surfline"))
	view.WriteString("\n\n")
	view.WriteString(ui.Prompt(m.Query))
	if !m.InResults() && !m.Loading {
		view.WriteString("_")
	}
	view.WriteString("\n\n")

	switch {
	case m.Loading:
		fmt.Fprintf(&view, "Searching Surfline for %q…\n", m.Query)
	case m.Err != nil:
		view.WriteString(ui.Error("Search failed: " + m.Err.Error()))
		view.WriteString("\n")
	case !m.HasSearched:
		view.WriteString(ui.Muted("Type a surf spot or location, then press Enter."))
		view.WriteString("\n")
	case len(m.Results) == 0:
		view.WriteString(ui.Muted("No matching Surfline spots found."))
		view.WriteString("\n")
	default:
		for index, spot := range m.Results {
			marker := "  "
			if index == m.Cursor {
				marker = "> "
			}
			location := strings.Trim(strings.Join([]string{spot.Region, spot.Country}, ", "), ", ")
			line := marker + spot.Name
			if location != "" {
				line += " — " + location
			}
			if index == m.Cursor {
				line = ui.Selected(line)
			}
			view.WriteString(line + "\n")
			if index == m.Cursor && spot.URL != "" {
				view.WriteString(ui.Muted("  " + spot.URL))
				view.WriteString("\n")
			}
		}
	}

	if m.Status != "" {
		view.WriteString("\n" + ui.Success(m.Status) + "\n")
	}

	view.WriteString("\n")
	if m.InResults() {
		view.WriteString(ui.Muted("↑/k ↓/j navigate • Enter track • Esc clear"))
	} else {
		view.WriteString(ui.Muted("Enter search • Esc clear/back • Ctrl+C quit"))
	}
	view.WriteString("\n")
	return view.String()
}
