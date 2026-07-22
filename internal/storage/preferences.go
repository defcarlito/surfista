package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const preferencesFileName = "settings.json"

type preferences struct {
	DashboardSort string `json:"dashboard_sort,omitempty"`
}

func (s *TrackedStore) preferencesPath() string {
	return filepath.Join(filepath.Dir(s.path), preferencesFileName)
}

func (s *TrackedStore) LoadSortMode() (string, error) {
	data, err := os.ReadFile(s.preferencesPath())
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read dashboard settings: %w", err)
	}

	var settings preferences
	if err := json.Unmarshal(data, &settings); err != nil {
		return "", fmt.Errorf("decode dashboard settings: %w", err)
	}
	return settings.DashboardSort, nil
}

func (s *TrackedStore) SaveSortMode(mode string) error {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return errors.New("cannot save an empty dashboard sort mode")
	}

	path := s.preferencesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create dashboard settings directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		return fmt.Errorf("create temporary dashboard settings file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(preferences{DashboardSort: mode}); err != nil {
		temporary.Close()
		return fmt.Errorf("encode dashboard settings: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync dashboard settings: %w", err)
	}
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect dashboard settings: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close dashboard settings: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace dashboard settings: %w", err)
	}
	return nil
}
