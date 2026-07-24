package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"surfista/internal/surf"
)

func TestTrackedStoreAddSaveAndPreventDuplicate(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "tracked.json")
	store := NewTrackedStore(path)
	spot := surf.Spot{
		ID:        "5842041f4e65fad6a7708827",
		Name:      "Huntington State Beach",
		URL:       "https://www.surfline.com/surf-report/huntington-state-beach/5842041f4e65fad6a7708827",
		Region:    "California, Orange County",
		Country:   "United States",
		Latitude:  33.654,
		Longitude: -118.003,
	}

	added, err := store.Add(spot)
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Fatal("first Add() = false, want true")
	}

	added, err = store.Add(spot)
	if err != nil {
		t.Fatal(err)
	}
	if added {
		t.Fatal("duplicate Add() = true, want false")
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0] != spot {
		t.Fatalf("Load() = %+v, want one saved spot", loaded)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var persisted []surf.Spot
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if len(persisted) != 1 || persisted[0].ID != spot.ID {
		t.Fatalf("saved JSON = %+v, want spot %q", persisted, spot.ID)
	}
}

func TestTrackedStoreRemovePersistsRemainingSpots(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "tracked.json")
	store := NewTrackedStore(path)
	first := surf.Spot{ID: "first", Name: "First"}
	second := surf.Spot{ID: "second", Name: "Second"}
	if _, err := store.Add(first); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(second); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveForecastCache(surf.ForecastCacheEntry{SpotID: first.ID}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveForecastCache(surf.ForecastCacheEntry{SpotID: second.ID}); err != nil {
		t.Fatal(err)
	}

	removed, err := store.Remove(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("Remove() = false, want true")
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0] != second {
		t.Fatalf("Load() after Remove() = %+v, want only second spot", loaded)
	}
	cached, err := store.LoadForecastCache()
	if err != nil {
		t.Fatal(err)
	}
	if len(cached) != 1 {
		t.Fatalf("cached locations after Remove() = %d, want 1", len(cached))
	}
	if _, exists := cached[first.ID]; exists {
		t.Fatal("removed favorite remained in forecast cache")
	}
	if _, exists := cached[second.ID]; !exists {
		t.Fatal("remaining favorite was removed from forecast cache")
	}

	removed, err = store.Remove("not-tracked")
	if err != nil {
		t.Fatal(err)
	}
	if removed {
		t.Fatal("Remove() missing spot = true, want false")
	}
	if _, err := store.Remove("  "); err == nil {
		t.Fatal("Remove() blank ID returned no error")
	}
}
