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

const forecastCacheFileName = "forecast-cache.json"

type forecastCacheFile struct {
	Version   int                                `json:"version"`
	Locations map[string]surf.ForecastCacheEntry `json:"locations"`
}

func (s *TrackedStore) forecastCachePath() string {
	return filepath.Join(filepath.Dir(s.path), forecastCacheFileName)
}

func (s *TrackedStore) LoadForecastCache() (map[string]surf.ForecastCacheEntry, error) {
	data, err := os.ReadFile(s.forecastCachePath())
	if errors.Is(err, os.ErrNotExist) {
		return map[string]surf.ForecastCacheEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read forecast cache: %w", err)
	}

	var cached forecastCacheFile
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("decode forecast cache: %w", err)
	}
	if cached.Locations == nil {
		cached.Locations = map[string]surf.ForecastCacheEntry{}
	}
	return cached.Locations, nil
}

func (s *TrackedStore) SaveForecastCache(entry surf.ForecastCacheEntry) error {
	spotID := strings.TrimSpace(entry.SpotID)
	if spotID == "" {
		return errors.New("cannot cache a forecast without a spot ID")
	}
	entry.SpotID = spotID

	locations, err := s.LoadForecastCache()
	if err != nil {
		return err
	}
	locations[spotID] = entry

	path := s.forecastCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create forecast cache directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".forecast-cache-*.json")
	if err != nil {
		return fmt.Errorf("create temporary forecast cache file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(forecastCacheFile{Version: 1, Locations: locations}); err != nil {
		temporary.Close()
		return fmt.Errorf("encode forecast cache: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync forecast cache: %w", err)
	}
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect forecast cache: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close forecast cache: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace forecast cache: %w", err)
	}
	return nil
}
