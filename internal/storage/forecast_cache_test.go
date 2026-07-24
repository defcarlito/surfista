package storage

import (
	"path/filepath"
	"testing"
	"time"

	"surfista/internal/surf"
)

func TestForecastCachePersistsAndMergesLocations(t *testing.T) {
	t.Parallel()

	store := NewTrackedStore(filepath.Join(t.TempDir(), "tracked.json"))
	firstUpdated := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	first := surf.ForecastCacheEntry{
		SpotID:            "first",
		Forecast:          surf.Forecast{SpotID: "first", Slots: []surf.ForecastSlot{{Rating: "Fair"}}},
		ForecastUpdatedAt: firstUpdated,
	}
	second := surf.ForecastCacheEntry{
		SpotID: "second",
		Details: surf.ForecastDetails{
			SpotID:   "second",
			Units:    surf.ForecastUnits{WindSpeed: "KTS"},
			Sunlight: []surf.SunlightDay{{Sunrise: firstUpdated, Sunset: firstUpdated.Add(12 * time.Hour)}},
		},
		DetailsUpdatedAt: firstUpdated.Add(time.Minute),
	}
	if err := store.SaveForecastCache(first); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveForecastCache(second); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.LoadForecastCache()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("cached locations = %d, want 2", len(loaded))
	}
	if got := loaded["first"]; got.ForecastUpdatedAt != firstUpdated || got.Forecast.Slots[0].Rating != "Fair" {
		t.Fatalf("first cached forecast = %+v", got)
	}
	if got := loaded["second"]; got.DetailsUpdatedAt != second.DetailsUpdatedAt || got.Details.Units.WindSpeed != "KTS" || len(got.Details.Sunlight) != 1 || !got.Details.Sunlight[0].Sunrise.Equal(firstUpdated) {
		t.Fatalf("second cached details = %+v", got)
	}
}

func TestForecastCacheRejectsMissingSpotID(t *testing.T) {
	t.Parallel()

	store := NewTrackedStore(filepath.Join(t.TempDir(), "tracked.json"))
	if err := store.SaveForecastCache(surf.ForecastCacheEntry{}); err == nil {
		t.Fatal("SaveForecastCache accepted an empty spot ID")
	}
}

func TestPruneForecastCacheRemovesLocationsThatAreNotFavorites(t *testing.T) {
	t.Parallel()

	store := NewTrackedStore(filepath.Join(t.TempDir(), "tracked.json"))
	for _, spotID := range []string{"first", "second", "stale"} {
		if err := store.SaveForecastCache(surf.ForecastCacheEntry{
			SpotID:            spotID,
			Forecast:          surf.Forecast{SpotID: spotID},
			ForecastUpdatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}

	favorites := []surf.Spot{{ID: "first"}, {ID: "second"}}
	if err := store.PruneForecastCache(favorites); err != nil {
		t.Fatal(err)
	}
	if err := store.PruneForecastCache(favorites); err != nil {
		t.Fatalf("idempotent prune failed: %v", err)
	}

	loaded, err := store.LoadForecastCache()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("cached locations after prune = %d, want 2", len(loaded))
	}
	if _, exists := loaded["stale"]; exists {
		t.Fatal("stale location remained in forecast cache")
	}
	if _, exists := loaded["first"]; !exists {
		t.Fatal("favorite location was removed from forecast cache")
	}
	if _, exists := loaded["second"]; !exists {
		t.Fatal("favorite location was removed from forecast cache")
	}
}
