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
