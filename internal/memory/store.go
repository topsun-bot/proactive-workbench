package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store is the local long-term memory API. Implementations must not call the network.
type Store interface {
	Load() (Snapshot, error)
	Save(Snapshot) error
}

// FileStore persists one Snapshot as JSON on disk. Writes are the whole document.
type FileStore struct {
	path string
	mu   sync.Mutex
}

func OpenFile(path string) (*FileStore, error) {
	if path == "" {
		return nil, fmt.Errorf("memory: file path is required")
	}
	if filepath.Ext(path) == "" {
		return nil, fmt.Errorf("memory: path %q must be a .json file", path)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("memory: mkdir: %w", err)
	}
	return &FileStore{path: path}, nil
}

func (s *FileStore) Path() string { return s.path }

func (s *FileStore) Load() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *FileStore) loadLocked() (Snapshot, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return Empty(), nil
		}
		return Snapshot{}, fmt.Errorf("memory: read: %w", err)
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, fmt.Errorf("memory: decode: %w", err)
	}
	snap.ensure()
	if err := snap.validate(); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

func (s *FileStore) Save(snap Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(snap)
}

func (s *FileStore) saveLocked(snap Snapshot) error {
	snap.ensure()
	if snap.Version == 0 {
		snap.Version = SchemaVersion
	}
	if err := snap.validate(); err != nil {
		return err
	}
	snap.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("memory: encode: %w", err)
	}
	data = append(data, '\n')
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("memory: write temp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("memory: rename: %w", err)
	}
	return nil
}

func (s *FileStore) Update(fn func(*Snapshot) error) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, err := s.loadLocked()
	if err != nil {
		return Snapshot{}, err
	}
	if err := fn(&snap); err != nil {
		return Snapshot{}, err
	}
	if err := s.saveLocked(snap); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

// SeedFixture writes the labeled fixture snapshot only when the file is empty
// or missing. It never overwrites a real store.
func (s *FileStore) SeedFixture() (Snapshot, error) {
	return s.Update(func(snap *Snapshot) error {
		if len(snap.Commitments)+len(snap.People)+len(snap.Preferences)+len(snap.Goals) > 0 {
			return nil
		}
		*snap = FixtureSnapshot()
		return nil
	})
}
