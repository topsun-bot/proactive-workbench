package memory

import "sync"

// MemStore is an in-process Snapshot. Used when no --memory-file is given.
type MemStore struct {
	mu   sync.Mutex
	snap Snapshot
}

func NewMemStore() *MemStore {
	return &MemStore{snap: Empty()}
}

func (s *MemStore) Load() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snap, nil
}

func (s *MemStore) Save(snap Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap.ensure()
	if err := snap.validate(); err != nil {
		return err
	}
	s.snap = snap
	return nil
}

func (s *MemStore) SeedFixture() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.snap.Commitments)+len(s.snap.People)+len(s.snap.Preferences)+len(s.snap.Goals) > 0 {
		return s.snap, nil
	}
	s.snap = FixtureSnapshot()
	return s.snap, nil
}

func (s *MemStore) Update(fn func(*Snapshot) error) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := s.snap
	if err := fn(&snap); err != nil {
		return Snapshot{}, err
	}
	snap.ensure()
	if err := snap.validate(); err != nil {
		return Snapshot{}, err
	}
	s.snap = snap
	return snap, nil
}
