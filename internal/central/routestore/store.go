package routestore

import (
	"net/netip"
	"sync"

	"github.com/gaissmai/bart"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
)

// Store manages per-LG-target prefix tries for route lookups.
type Store struct {
	mu    sync.RWMutex
	tries map[model.LGTargetID]*lgTargetTrie
}

type lgTargetTrie struct {
	mu    sync.RWMutex
	trie  bart.Table[*model.RouteEntry]
	count uint32
}

// New creates an empty Store.
func New() *Store {
	return &Store{
		tries: make(map[model.LGTargetID]*lgTargetTrie),
	}
}

// CreateTrie creates a new empty trie for an LG target.
// If a trie already exists, it is replaced.
func (s *Store) CreateTrie(id model.LGTargetID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tries[id] = &lgTargetTrie{}
}

// DeleteTrie removes the trie for an LG target.
func (s *Store) DeleteTrie(id model.LGTargetID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tries, id)
}

// getTrie returns the trie for an LG target, or nil if not found.
func (s *Store) getTrie(id model.LGTargetID) *lgTargetTrie {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tries[id]
}

// LookupExact returns all BGP paths for an exact prefix match on the given LG target.
func (s *Store) LookupExact(id model.LGTargetID, prefix netip.Prefix) *model.RouteEntry {
	t := s.getTrie(id)
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	entry, ok := t.trie.Get(prefix)
	if !ok {
		return nil
	}
	return entry
}

// LookupLPM returns the most specific route entry covering the given IP address.
func (s *Store) LookupLPM(id model.LGTargetID, addr netip.Addr) *model.RouteEntry {
	t := s.getTrie(id)
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	entry, ok := t.trie.Lookup(addr)
	if !ok {
		return nil
	}
	return entry
}

// UpsertRoute adds or updates a route path for a prefix on the given LG target.
func (s *Store) UpsertRoute(id model.LGTargetID, prefix netip.Prefix, path model.BGPPath) {
	t := s.getTrie(id)
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	existing, ok := t.trie.Get(prefix)
	if !ok {
		// New entry
		entry := &model.RouteEntry{
			Prefix: prefix,
			Paths:  []model.BGPPath{path},
		}
		t.trie.Insert(prefix, entry)
		t.count++
		return
	}

	// Update existing entry - replace path with same PathID or append
	for i, p := range existing.Paths {
		if p.PathID == path.PathID && path.PathID != 0 {
			existing.Paths[i] = path
			return
		}
	}
	existing.Paths = append(existing.Paths, path)
}

// WithdrawRoute removes a specific path from a prefix on the given LG target.
// If no paths remain after withdrawal, the prefix is removed from the trie.
func (s *Store) WithdrawRoute(id model.LGTargetID, prefix netip.Prefix, pathID uint32) {
	t := s.getTrie(id)
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	existing, ok := t.trie.Get(prefix)
	if !ok {
		return
	}

	if pathID == 0 {
		// No AddPath - remove all paths (legacy single-path behavior)
		t.trie.Delete(prefix)
		t.count--
		return
	}

	// Remove the specific path
	paths := existing.Paths[:0]
	for _, p := range existing.Paths {
		if p.PathID != pathID {
			paths = append(paths, p)
		}
	}

	if len(paths) == 0 {
		t.trie.Delete(prefix)
		t.count--
	} else {
		existing.Paths = paths
	}
}

// ReplaceAll atomically replaces all routes for an LG target.
// Used during snapshot processing.
func (s *Store) ReplaceAll(id model.LGTargetID, entries map[netip.Prefix]*model.RouteEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	newTrie := &lgTargetTrie{}
	for prefix, entry := range entries {
		newTrie.trie.Insert(prefix, entry)
		newTrie.count++
	}

	s.tries[id] = newTrie
}

// Count returns the number of prefixes stored for an LG target.
func (s *Store) Count(id model.LGTargetID) uint32 {
	t := s.getTrie(id)
	if t == nil {
		return 0
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.count
}

// TotalRoutes returns the sum of all routes across all LG targets.
func (s *Store) TotalRoutes() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total uint64
	for _, t := range s.tries {
		t.mu.RLock()
		total += uint64(t.count)
		t.mu.RUnlock()
	}
	return total
}
