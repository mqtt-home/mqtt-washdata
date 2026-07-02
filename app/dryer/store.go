package dryer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/philipparndt/go-logger"
)

// Store persists completed runs as one JSON file per run under a directory.
type Store struct {
	dir  string
	mu   sync.RWMutex
	runs map[string]*Run
}

// NewStore creates a store rooted at dir (created if missing).
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create runs dir: %w", err)
	}
	return &Store{dir: dir, runs: make(map[string]*Run)}, nil
}

// Load reads all run files from disk into memory.
func (s *Store) Load() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("read runs dir: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			logger.Warn("Failed to read run file", "path", path, "error", err)
			continue
		}
		var r Run
		if err := json.Unmarshal(data, &r); err != nil {
			logger.Warn("Failed to parse run file", "path", path, "error", err)
			continue
		}
		s.runs[r.ID] = &r
	}
	logger.Info("Loaded dryer runs", "count", len(s.runs))
	return nil
}

// Save writes a run to disk and updates the in-memory index.
func (s *Store) Save(r *Run) error {
	s.mu.Lock()
	s.runs[r.ID] = r
	s.mu.Unlock()

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal run: %w", err)
	}
	path := filepath.Join(s.dir, r.ID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write run: %w", err)
	}
	return nil
}

// Delete removes a run from disk and memory.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	delete(s.runs, id)
	s.mu.Unlock()

	path := filepath.Join(s.dir, id+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete run: %w", err)
	}
	return nil
}

// Get returns a run by id.
func (s *Store) Get(id string) (*Run, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.runs[id]
	return r, ok
}

// All returns every stored run, newest first.
func (s *Store) All() []*Run {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Run, 0, len(s.runs))
	for _, r := range s.runs {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Start.After(out[j].Start)
	})
	return out
}
