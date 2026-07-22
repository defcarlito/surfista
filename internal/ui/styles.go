package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	SearchInputFrameWidth = 4
	LoadingBanner         = "                    _____      __       \n" +
		"   _______  _______/ __(_)____/ /_____ _\n" +
		"  / ___/ / / / ___/ /_/ / ___/ __/ __ `/\n" +
		" (__  ) /_/ / /  / __/ (__  ) /_/ /_/ / \n" +
		"/____/\\__,_/_/  /_/ /_/____/\\__/\\__,_/  "
	SearchBanner = "                              __                         _______          \n" +
		"   ________  ____ ___________/ /_       _______  _______/ __/ (_)___  ___ \n" +
		"  / ___/ _ \\/ __ `/ ___/ ___/ __ \\     / ___/ / / / ___/ /_/ / / __ \\/ _ \\\n" +
		" (__  )  __/ /_/ / /  / /__/ / / /    (__  ) /_/ / /  / __/ / / / / /  __/\n" +
		"/____/\\___/\\__,_/_/   \\___/_/ /_/    /____/\\__,_/_/  /_/ /_/_/_/ /_/\\___/"
)

var (
	OceanPalette = struct {
		White   color.Color
		Foam    color.Color
		Current color.Color
		Depth   color.Color
	}{
		White:   lipgloss.Color("#FFFFFF"),
		Foam:    lipgloss.Color("#ACECF7"),
		Current: lipgloss.Color("#4056F4"),
		Depth:   lipgloss.Color("#011936"),
	}

	primaryColor = OceanPalette.Foam
	mutedColor   = lipgloss.Color("#7189A8")
	borderColor  = OceanPalette.Current
	successColor = lipgloss.Color("2")
	errorColor   = lipgloss.Color("1")

	ratingVeryPoorColor = lipgloss.Color("#F0446D")
	ratingPoorColor     = lipgloss.Color("#FF9100")
	ratingPoorFairColor = lipgloss.Color("#FFCA28")
	ratingFairColor     = lipgloss.Color("#16CC77")
	ratingFairGoodColor = lipgloss.Color("#0D9E86")
	ratingGoodColor     = lipgloss.Color("#684CF3")
	ratingVeryGoodColor = lipgloss.Color("#6C36E5")
	ratingEpicColor     = lipgloss.Color("#7020D6")

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
				BorderForeground(borderColor).
				Padding(0, 1)

	SearchPlaceholderStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	SearchTextStyle = lipgloss.NewStyle()

	SearchResultStyle = lipgloss.NewStyle().
				PaddingLeft(1)

	SearchURLStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(mutedColor).
			PaddingLeft(2)

	SearchSpinnerStyle = lipgloss.NewStyle().
				Foreground(primaryColor)

	LoadingStatusStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Align(lipgloss.Center)

	SearchHelpStyle = lipgloss.NewStyle().
			Faint(true).
			Foreground(mutedColor)

	SearchEmptyStyle = lipgloss.NewStyle().
				Foreground(mutedColor)

	DashboardSubtitleStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Align(lipgloss.Center)

	DashboardSpotStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor)

	DashboardSlotStyle = lipgloss.NewStyle().
				Align(lipgloss.Center)

	DashboardCurrentHourStyle = DashboardSlotStyle.
					Bold(true).
					Foreground(OceanPalette.White).
					Background(borderColor)

	DashboardNowStyle = lipgloss.NewStyle().
				Foreground(OceanPalette.Foam).
				Align(lipgloss.Center)

	DashboardBorderStyle = lipgloss.NewStyle().
				Foreground(borderColor)

	DashboardSelectedBorderStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(OceanPalette.White)

	DashboardRemovalDialogStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(borderColor).
					Foreground(OceanPalette.White).
					Padding(1, 2)

	DashboardRemovalBodyStyle = lipgloss.NewStyle().
					Foreground(OceanPalette.White)

	DashboardRemovalHelpStyle = lipgloss.NewStyle().
					Faint(true).
					Foreground(mutedColor)

	DashboardDetailDialogStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(borderColor).
					Foreground(OceanPalette.White).
					Padding(1, 1)

	DashboardDetailTitleStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(primaryColor)

	DashboardDetailLabelStyle = lipgloss.NewStyle().
					Faint(true).
					Foreground(mutedColor)

	DashboardDetailValueStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(OceanPalette.White)

	DashboardDetailHelpStyle = lipgloss.NewStyle().
					Faint(true).
					Foreground(mutedColor)

	DashboardHelpStyle = lipgloss.NewStyle().
				Faint(true).
				Foreground(mutedColor)

	DashboardSortStyle = lipgloss.NewStyle().
				Foreground(OceanPalette.White)

	DashboardScrollIndicatorStyle = lipgloss.NewStyle().
					Foreground(primaryColor).
					Align(lipgloss.Center)

	DashboardEmptyStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Align(lipgloss.Center)
)

func Title(value string) string   { return TitleStyle.Render(GradientText(value)) }
func Muted(value string) string   { return MutedStyle.Render(value) }
func Success(value string) string { return SuccessStyle.Render(value) }
func Error(value string) string   { return ErrorStyle.Render(value) }

func DashboardRating(value, rating string) string {
	var foreground color.Color
	switch rating {
	case "Very Poor":
		foreground = ratingVeryPoorColor
	case "Poor":
		foreground = ratingPoorColor
	case "Poor to Fair":
		foreground = ratingPoorFairColor
	case "Fair":
		foreground = ratingFairColor
	case "Fair to Good":
		foreground = ratingFairGoodColor
	case "Good":
		foreground = ratingGoodColor
	case "Very Good":
		foreground = ratingVeryGoodColor
	case "Epic":
		foreground = ratingEpicColor
	}
	if foreground == nil {
		return value
	}
	return lipgloss.NewStyle().Foreground(foreground).Render(value)
}

func GradientText(value string) string {
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		lines[index] = gradientTextLine(line)
	}
	return strings.Join(lines, "\n")
}

func gradientTextLine(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}

	colors := lipgloss.Blend1D(len(runes), oceanGradientStops()...)
	var gradient strings.Builder
	for index, character := range runes {
		gradient.WriteString(lipgloss.NewStyle().Foreground(colors[index]).Render(string(character)))
	}
	return gradient.String()
}

func GradientBackground(value string, width int) string {
	if width <= 0 {
		return ""
	}

	value = ansi.Truncate(value, width, "")
	if short := width - ansi.StringWidth(value); short > 0 {
		value += strings.Repeat(" ", short)
	}

	colors := lipgloss.Blend1D(width, oceanGradientStops()...)
	var gradient strings.Builder
	cell := 0
	for _, character := range value {
		characterWidth := max(1, ansi.StringWidth(string(character)))
		background := colors[min(cell, len(colors)-1)]
		gradient.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(contrastingText(background)).
			Background(background).
			Render(string(character)))
		cell += characterWidth
	}
	return gradient.String()
}

func oceanGradientStops() []color.Color {
	return []color.Color{
		OceanPalette.White,
		OceanPalette.Foam,
		OceanPalette.Current,
		OceanPalette.Depth,
	}
}

func contrastingText(background color.Color) color.Color {
	red, green, blue, _ := background.RGBA()
	luminance := (0.2126 * float64(red)) + (0.7152 * float64(green)) + (0.0722 * float64(blue))
	if luminance > 0.5*0xffff {
		return OceanPalette.Depth
	}
	return OceanPalette.White
}
