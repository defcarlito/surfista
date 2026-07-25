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

	model := New(1, true)
	model.SetProgress(0)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	rendered := model.View()
	plain := ansi.Strip(rendered)

	for _, bannerLine := range strings.Split(ansi.Strip(ui.LoadingBanner), "\n") {
		if !strings.Contains(plain, strings.TrimSpace(bannerLine)) {
			t.Fatalf("view does not contain banner line %q:\n%s", bannerLine, plain)
		}
	}
	if !strings.Contains(plain, "Fetching forecasts 0/1") {
		t.Fatalf("view does not contain forecast progress:\n%s", plain)
	}
	lines := strings.Split(plain, "\n")
	if got := strings.TrimSpace(lines[len(lines)-1]); got != "enter use cache while updates continue" {
		t.Fatalf("bottom help = %q, want skip-cache control:\n%s", got, plain)
	}
	if !strings.Contains(rendered, "\x1b[") {
		t.Fatal("loading banner was rendered without gradient styling")
	}
}

func TestViewShowsSingleForecastProgressPhase(t *testing.T) {
	t.Parallel()

	model := New(5, false)
	model.SetProgress(2)
	forecastsView := ansi.Strip(model.View())
	if !strings.Contains(forecastsView, "Fetching forecasts 2/5") {
		t.Fatalf("forecast progress is missing:\n%s", forecastsView)
	}
	if strings.Contains(forecastsView, "Locations loaded") {
		t.Fatalf("removed location-loading phase is still visible:\n%s", forecastsView)
	}
	if strings.Contains(forecastsView, "updates continue") {
		t.Fatalf("skip control appeared without cached data:\n%s", forecastsView)
	}
}
