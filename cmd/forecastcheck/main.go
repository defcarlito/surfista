// Command forecastcheck exercises Surfista's live forecast provider against
// every saved favorite without starting the TUI.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"surfista/internal/storage"
	"surfista/internal/surf"
)

func main() {
	store, err := storage.NewDefaultTrackedStore()
	if err != nil {
		fatal("find saved favorites", err)
	}

	spots, err := store.Load()
	if err != nil {
		fatal("load saved favorites", err)
	}
	if len(spots) == 0 {
		fmt.Println("No favorite surf spots are saved.")
		return
	}

	provider, err := surf.NewSurflineForecastProvider(
		"",
		&http.Client{Timeout: 20 * time.Second},
	)
	if err != nil {
		fatal("configure Surfline forecast provider", err)
	}

	failures := 0
	for _, spot := range spots {
		fmt.Printf("\n%s (%s)\n", spot.Name, spot.ID)

		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		forecast, err := provider.Forecast(ctx, spot.ID)
		cancel()
		if err != nil {
			failures++
			fmt.Printf("  ERROR: %v\n", err)
			continue
		}

		spotTimezone := time.FixedZone("spot", int(forecast.UTCOffset/time.Second))
		for _, slot := range forecast.Slots {
			height := fmt.Sprintf("%g-%g ft", slot.SurfHeight.Min, slot.SurfHeight.Max)
			if slot.SurfHeight.Plus {
				height += "+"
			}
			if slot.SurfHeight.HumanRelation != "" {
				height += " (" + slot.SurfHeight.HumanRelation + ")"
			}
			fmt.Printf("  %-8s  %-12s  %s\n",
				slot.Timestamp.In(spotTimezone).Format("3:04 PM"),
				slot.Rating,
				height,
			)
		}
	}

	if failures > 0 {
		fmt.Fprintf(os.Stderr, "\n%d of %d favorites failed.\n", failures, len(spots))
		os.Exit(1)
	}
	fmt.Printf("\nAll %d favorites returned matching wave and rating data.\n", len(spots))
}

func fatal(action string, err error) {
	fmt.Fprintf(os.Stderr, "Could not %s: %v\n", action, err)
	os.Exit(1)
}
