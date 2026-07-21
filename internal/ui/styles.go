package ui

import "charm.land/lipgloss/v2"

const SearchInputFrameWidth = 4

var (
	primaryColor  = lipgloss.Color("6")
	mutedColor    = lipgloss.Color("8")
	successColor  = lipgloss.Color("2")
	errorColor    = lipgloss.Color("1")
	selectedColor = lipgloss.Color("0")

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	MutedStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(mutedColor)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(successColor)

	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(errorColor)

	SearchTitleStyle = TitleStyle.Align(lipgloss.Center)

	SearchInputStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(mutedColor).
				Padding(0, 1)

	SearchPlaceholderStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	SearchTextStyle = lipgloss.NewStyle()

	SearchResultStyle = lipgloss.NewStyle().
				PaddingLeft(1)

	SearchSelectedResultStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(selectedColor).
					Background(primaryColor).
					PaddingLeft(1)

	SearchURLStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(mutedColor).
			PaddingLeft(2)

	SearchSpinnerStyle = lipgloss.NewStyle().
				Foreground(primaryColor)

	SearchHelpStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(mutedColor)

	SearchEmptyStyle = lipgloss.NewStyle().
				Foreground(mutedColor)
)

func Title(value string) string   { return TitleStyle.Render(value) }
func Muted(value string) string   { return MutedStyle.Render(value) }
func Success(value string) string { return SuccessStyle.Render(value) }
func Error(value string) string   { return ErrorStyle.Render(value) }
