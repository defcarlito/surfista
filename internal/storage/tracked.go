package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"surfista/internal/surf"
)

type TrackedStore struct {
	path string
}

func NewDefaultTrackedStore() (*TrackedStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return NewTrackedStore(filepath.Join(configDir, "surfista", "tracked.json")), nil
}

func NewTrackedStore(path string) *TrackedStore {
	return &TrackedStore{path: path}
}

func (s *TrackedStore) Load() ([]surf.Spot, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []surf.Spot{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read tracked locations: %w", err)
	}

	var spots []surf.Spot
	if err := json.Unmarshal(data, &spots); err != nil {
		return nil, fmt.Errorf("decode tracked locations: %w", err)
	}
	return spots, nil
}

// Add persists spot unless its stable provider ID is already tracked. It
// returns true only when a new spot was saved.
func (s *TrackedStore) Add(spot surf.Spot) (bool, error) {
	if strings.TrimSpace(spot.ID) == "" {
		return false, errors.New("cannot track a spot without an ID")
	}

	spots, err := s.Load()
	if err != nil {
		return false, err
	}
	for _, tracked := range spots {
		if tracked.ID == spot.ID {
			return false, nil
		}
	}

	spots = append(spots, spot)
	if err := s.save(spots); err != nil {
		return false, err
	}
	return true, nil
}

func (s *TrackedStore) save(spots []surf.Spot) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create tracked locations directory: %w", err)
	}

	temporary, err := os.CreateTemp(dir, ".tracked-*.json")
	if err != nil {
		return fmt.Errorf("create temporary tracked locations file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(spots); err != nil {
		temporary.Close()
		return fmt.Errorf("encode tracked locations: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync tracked locations: %w", err)
	}
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect tracked locations: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close tracked locations: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("replace tracked locations: %w", err)
	}

	return nil
}
