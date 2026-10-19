package config

import "sync"

type Store struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

func NewStore(path string, cfg Config) *Store {
	return &Store{path: path, cfg: cfg}
}

func (s *Store) Snapshot() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *Store) Update(cfg Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := Save(s.path, cfg); err != nil {
		return err
	}
	s.cfg = cfg
	return nil
}
