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

	model := New(1)
	model.SetProgress(1, 0)
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
	if !strings.Contains(rendered, "\x1b[") {
		t.Fatal("loading banner was rendered without gradient styling")
	}
}

func TestViewSwitchesFromLocationsToForecasts(t *testing.T) {
	t.Parallel()

	model := New(5)
	model.SetProgress(3, 2)
	locationsView := ansi.Strip(model.View())
	if !strings.Contains(locationsView, "Locations loaded 3/5") {
		t.Fatalf("location phase does not contain progress:\n%s", locationsView)
	}
	if strings.Contains(locationsView, "Fetching forecasts") {
		t.Fatalf("forecast phase appeared before locations finished:\n%s", locationsView)
	}

	model.SetProgress(5, 2)
	forecastsView := ansi.Strip(model.View())
	if !strings.Contains(forecastsView, "Fetching forecasts 2/5") {
		t.Fatalf("forecast phase does not contain progress:\n%s", forecastsView)
	}
	if strings.Contains(forecastsView, "Locations loaded") {
		t.Fatalf("location phase remained after locations finished:\n%s", forecastsView)
	}
}
