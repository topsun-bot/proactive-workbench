package proactivity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const lastRunSchemaVersion = 1

type lastRunDoc struct {
	Version int               `json:"version"`
	LastRun map[string]string `json:"lastRun"`
}

// loadLastRun reads path. A missing or corrupt file is treated as empty
// (every routine is due). It never returns an error that should stop the core.
func loadLastRun(path string) map[RoutineKind]time.Time {
	out := map[RoutineKind]time.Time{}
	if path == "" {
		return out
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var doc lastRunDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return out
	}
	if doc.Version != lastRunSchemaVersion {
		return out
	}
	for k, raw := range doc.LastRun {
		if raw == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			continue
		}
		out[RoutineKind(k)] = t
	}
	return out
}

func saveLastRun(path string, last map[RoutineKind]time.Time) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("proactivity: mkdir last-run: %w", err)
	}
	doc := lastRunDoc{
		Version: lastRunSchemaVersion,
		LastRun: map[string]string{},
	}
	for k, t := range last {
		if t.IsZero() {
			continue
		}
		doc.LastRun[string(k)] = t.Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("proactivity: encode last-run: %w", err)
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("proactivity: write last-run temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("proactivity: rename last-run: %w", err)
	}
	return nil
}
