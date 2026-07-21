package loading

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"surfista/internal/ui"
)

func TestViewShowsGradientBannerAndFetchProgress(t *testing.T) {
	t.Parallel()

	model := New(2)
	model.SetCompleted(1)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	rendered := model.View()
	plain := ansi.Strip(rendered)

	for _, bannerLine := range strings.Split(ansi.Strip(ui.LoadingBanner), "\n") {
		if !strings.Contains(plain, strings.TrimSpace(bannerLine)) {
			t.Fatalf("view does not contain banner line %q:\n%s", bannerLine, plain)
		}
	}
	if !strings.Contains(plain, "Fetching favorite forecasts… 1/2") {
		t.Fatalf("view does not contain fetch progress:\n%s", plain)
	}
	if !strings.Contains(rendered, "\x1b[") {
		t.Fatal("loading banner was rendered without gradient styling")
	}
}
