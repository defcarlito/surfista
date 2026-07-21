package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	"surfista/internal/app"
	"surfista/internal/storage"
	"surfista/internal/surf"
)

func main() {
	store, err := storage.NewDefaultTrackedStore()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error finding the local data directory:", err)
		os.Exit(1)
	}

	tracked, loadErr := store.Load()
	searcher, err := surf.NewSitemapSearcher(
		os.Getenv("SURFISTA_SPOT_SITEMAP_URL"),
		&http.Client{Timeout: 20 * time.Second},
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error configuring spot search:", err)
		os.Exit(1)
	}
	forecaster, err := surf.NewSurflineForecastProvider(
		os.Getenv("SURFISTA_SURFLINE_BASE_URL"),
		&http.Client{Timeout: 20 * time.Second},
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error configuring forecasts:", err)
		os.Exit(1)
	}

	program := tea.NewProgram(app.New(searcher, store, forecaster, tracked, loadErr))
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
