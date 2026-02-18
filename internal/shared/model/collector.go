package model

import (
	"sync"
	"time"
)

type CollectorStatus string

const (
	CollectorStatusConnecting   CollectorStatus = "connecting"
	CollectorStatusConnected    CollectorStatus = "connected"
	CollectorStatusSyncing      CollectorStatus = "syncing"
	CollectorStatusActive       CollectorStatus = "active"
	CollectorStatusDisconnected CollectorStatus = "disconnected"
)

type Collector struct {
	ID             string                    `json:"id"`
	Location       string                    `json:"location"`
	Status         CollectorStatus           `json:"status"`
	ConnectedAt    time.Time                 `json:"connected_at"`
	DisconnectedAt time.Time                 `json:"disconnected_at,omitempty"`
	RouterSessions map[string]*RouterSession `json:"-"`
	mu             sync.RWMutex              `json:"-"`
}

func (c *Collector) SetStatus(s CollectorStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Status = s
}

func (c *Collector) GetStatus() CollectorStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Status
}

func (c *Collector) GetRouterSession(id string) *RouterSession {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.RouterSessions[id]
}

func (c *Collector) ListRouterSessions() []*RouterSession {
	c.mu.RLock()
	defer c.mu.RUnlock()
	sessions := make([]*RouterSession, 0, len(c.RouterSessions))
	for _, s := range c.RouterSessions {
		sessions = append(sessions, s)
	}
	return sessions
}

// SetConnectionInfo atomically updates connection-related fields on reconnect.
func (c *Collector) SetConnectionInfo(location string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ConnectedAt = time.Now()
	c.Location = location
}

// ResetSessions atomically replaces the router sessions map with a new empty
// map and returns the old sessions for cleanup by the caller.
func (c *Collector) ResetSessions() map[string]*RouterSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	old := c.RouterSessions
	c.RouterSessions = make(map[string]*RouterSession)
	return old
}

// AddSession adds a router session under the collector's lock.
func (c *Collector) AddSession(id string, sess *RouterSession) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.RouterSessions[id] = sess
}

// MarkDisconnected atomically marks the collector and all its sessions as disconnected.
func (c *Collector) MarkDisconnected() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Status = CollectorStatusDisconnected
	c.DisconnectedAt = time.Now()
	for _, sess := range c.RouterSessions {
		sess.SetStatus(RouterSessionStatusDown)
	}
}
