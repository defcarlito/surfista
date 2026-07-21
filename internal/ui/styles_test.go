package ui

import (
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
