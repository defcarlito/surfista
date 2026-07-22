package ui

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestGradientTextPreservesContent(t *testing.T) {
	t.Parallel()

	const content = "Search Surfline"
	rendered := GradientText(content)
	if plain := ansi.Strip(rendered); plain != content {
		t.Fatalf("gradient text = %q, want %q", plain, content)
	}
	if !strings.Contains(rendered, "\x1b[") {
		t.Fatal("gradient text contains no terminal styling")
	}
}

func TestGradientTextPreservesMultilineBanner(t *testing.T) {
	t.Parallel()

	rendered := GradientText(SearchBanner)
	if plain := ansi.Strip(rendered); plain != SearchBanner {
		t.Fatal("gradient changed the ASCII banner content")
	}
}

func TestGradientBackgroundFillsRequestedWidth(t *testing.T) {
	t.Parallel()

	rendered := GradientBackground("> Honolua Bay", 30)
	if width := lipgloss.Width(rendered); width != 30 {
		t.Fatalf("gradient background width = %d, want 30", width)
	}
	if plain := ansi.Strip(rendered); !strings.HasPrefix(plain, "> Honolua Bay") {
		t.Fatalf("gradient background lost its content: %q", plain)
	}
}

func TestDashboardRatingUsesConditionColorScale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		rating string
		color  color.Color
	}{
		{rating: "Very Poor", color: lipgloss.Color("#F0446D")},
		{rating: "Poor", color: lipgloss.Color("#FF9100")},
		{rating: "Poor to Fair", color: lipgloss.Color("#FFCA28")},
		{rating: "Fair", color: lipgloss.Color("#16CC77")},
		{rating: "Fair to Good", color: lipgloss.Color("#0D9E86")},
		{rating: "Good", color: lipgloss.Color("#684CF3")},
		{rating: "Very Good", color: lipgloss.Color("#6C36E5")},
		{rating: "Epic", color: lipgloss.Color("#7020D6")},
	}

	for _, test := range tests {
		test := test
		t.Run(test.rating, func(t *testing.T) {
			t.Parallel()
			got := DashboardRating("rating", test.rating)
			want := lipgloss.NewStyle().Foreground(test.color).Render("rating")
			if got != want {
				t.Fatalf("DashboardRating(%q) = %q, want %q", test.rating, got, want)
			}
			if ansi.Strip(got) != "rating" {
				t.Fatalf("rating color changed text content: %q", got)
			}
		})
	}

	if got := DashboardRating("Unknown", "UNKNOWN"); got != "Unknown" {
		t.Fatalf("unknown rating = %q, want unstyled text", got)
	}
}
