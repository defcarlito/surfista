package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"surfista/internal/ui"
)

func (m Model) View() tea.View {
	content := m.homeView()
	if m.current == searchScreen {
		content = m.search.View()
	}

	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "Surfista"
	return view
}

func (m Model) homeView() string {
	var view strings.Builder
	view.WriteString(ui.Title("Surfista"))
	view.WriteString("\n\n")

	if m.loadErr != nil {
		view.WriteString(ui.Error("Could not load tracked locations: " + m.loadErr.Error()))
		view.WriteString("\n\n")
	}

	if len(m.tracked) == 0 {
		view.WriteString(ui.Muted("No tracked surf spots yet."))
		view.WriteString("\n")
	} else {
		view.WriteString("Tracked locations\n")
		for _, spot := range m.tracked {
			location := strings.Trim(strings.Join([]string{spot.Region, spot.Country}, ", "), ", ")
			fmt.Fprintf(&view, "  • %s", spot.Name)
			if location != "" {
				fmt.Fprintf(&view, " — %s", location)
			}
			view.WriteString("\n")
			if spot.URL != "" {
				fmt.Fprintf(&view, "    %s\n", ui.Muted(spot.URL))
			}
		}
	}

	view.WriteString("\n")
	view.WriteString(ui.Muted("s or / search Surfline • q quit"))
	view.WriteString("\n")
	return view.String()
}
