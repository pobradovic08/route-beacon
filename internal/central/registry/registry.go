package registry

import (
	"fmt"
	"sync"
	"time"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
)

// Registry manages the set of known collectors and their router sessions.
type Registry struct {
	mu          sync.RWMutex
	collectors  map[string]*model.Collector
	lgTargetIdx map[model.LGTargetID]*model.RouterSession
}

// New creates an empty Registry.
func New() *Registry {
	return &Registry{
		collectors:  make(map[string]*model.Collector),
		lgTargetIdx: make(map[model.LGTargetID]*model.RouterSession),
	}
}

// RegisterCollector adds or updates a collector and its router sessions.
func (r *Registry) RegisterCollector(id, location string, sessions []*model.RouterSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	collector, exists := r.collectors[id]
	if !exists {
		collector = &model.Collector{
			ID:             id,
			Location:       location,
			Status:         model.CollectorStatusConnected,
			ConnectedAt:    time.Now(),
			RouterSessions: make(map[string]*model.RouterSession),
		}
		r.collectors[id] = collector
	} else {
		collector.SetStatus(model.CollectorStatusConnected)
		collector.SetConnectionInfo(location)
		// Remove old LG target index entries
		old := collector.ResetSessions()
		for _, sess := range old {
			delete(r.lgTargetIdx, sess.LGTargetID())
		}
	}

	for _, sess := range sessions {
		sess.CollectorID = id
		collector.AddSession(sess.ID, sess)
		r.lgTargetIdx[sess.LGTargetID()] = sess
	}

	return nil
}

// UnregisterCollector marks a collector as disconnected.
func (r *Registry) UnregisterCollector(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	collector, exists := r.collectors[id]
	if !exists {
		return
	}
	collector.MarkDisconnected()
}

// UpdateCollectorStatus updates the status of a collector.
func (r *Registry) UpdateCollectorStatus(id string, status model.CollectorStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	collector, exists := r.collectors[id]
	if !exists {
		return fmt.Errorf("collector %q not found", id)
	}
	collector.SetStatus(status)
	return nil
}

// GetCollector returns a collector by ID.
func (r *Registry) GetCollector(id string) *model.Collector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.collectors[id]
}

// ListCollectors returns all registered collectors.
func (r *Registry) ListCollectors() []*model.Collector {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*model.Collector, 0, len(r.collectors))
	for _, c := range r.collectors {
		result = append(result, c)
	}
	return result
}

// GetTarget returns a router session by its LG target ID.
func (r *Registry) GetTarget(id model.LGTargetID) *model.RouterSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lgTargetIdx[id]
}

// GetTargetByString returns a router session by its string-form target ID.
// The targetId is expected to be in "collectorId:sessionId" format.
func (r *Registry) GetTargetByString(targetId string) *model.RouterSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for id, sess := range r.lgTargetIdx {
		if id.String() == targetId {
			return sess
		}
	}
	return nil
}

// ListTargets returns all LG targets across all collectors.
func (r *Registry) ListTargets() []*model.RouterSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*model.RouterSession, 0, len(r.lgTargetIdx))
	for _, sess := range r.lgTargetIdx {
		result = append(result, sess)
	}
	return result
}

// ListTargetsByCollector returns all LG targets for a specific collector.
func (r *Registry) ListTargetsByCollector(collectorID string) []*model.RouterSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	collector, exists := r.collectors[collectorID]
	if !exists {
		return nil
	}
	return collector.ListRouterSessions()
}

// UpdateSessionStatus updates the status of a router session.
func (r *Registry) UpdateSessionStatus(target model.LGTargetID, status model.RouterSessionStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	sess, exists := r.lgTargetIdx[target]
	if !exists {
		return fmt.Errorf("target %s not found", target)
	}
	sess.SetStatus(status)
	return nil
}

// UpdateSessionLastUpdate updates the last update timestamp for a session.
func (r *Registry) UpdateSessionLastUpdate(target model.LGTargetID, t time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sess, exists := r.lgTargetIdx[target]; exists {
		sess.SetLastUpdate(t)
	}
}

// UpdateSessionRouteCount updates the route count for a session.
func (r *Registry) UpdateSessionRouteCount(target model.LGTargetID, count uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if sess, exists := r.lgTargetIdx[target]; exists {
		sess.SetRouteCount(count)
	}
}

// ConnectedCollectorCount returns the number of currently connected collectors.
func (r *Registry) ConnectedCollectorCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, c := range r.collectors {
		if c.GetStatus() != model.CollectorStatusDisconnected {
			count++
		}
	}
	return count
}

// TotalCollectorCount returns the total number of registered collectors.
func (r *Registry) TotalCollectorCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.collectors)
}
