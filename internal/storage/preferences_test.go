package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSortModePersistsAcrossStoreInstances(t *testing.T) {
	t.Parallel()

	trackedPath := filepath.Join(t.TempDir(), "surfista", "tracked.json")
	store := NewTrackedStore(trackedPath)
	mode, err := store.LoadSortMode()
	if err != nil || mode != "" {
		t.Fatalf("initial LoadSortMode() = %q, %v; want empty mode", mode, err)
	}

	if err := store.SaveSortMode("condition_high_to_low"); err != nil {
		t.Fatal(err)
	}
	restartedStore := NewTrackedStore(trackedPath)
	mode, err = restartedStore.LoadSortMode()
	if err != nil || mode != "condition_high_to_low" {
		t.Fatalf("restarted LoadSortMode() = %q, %v", mode, err)
	}

	info, err := os.Stat(filepath.Join(filepath.Dir(trackedPath), preferencesFileName))
	if err != nil {
		t.Fatal(err)
	}
	if permissions := info.Mode().Perm(); permissions != 0o600 {
		t.Fatalf("settings permissions = %o, want 600", permissions)
	}
	if err := store.SaveSortMode("  "); err == nil {
		t.Fatal("SaveSortMode accepted an empty mode")
	}
}
