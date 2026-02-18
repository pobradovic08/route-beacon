package model

import (
	"net/netip"
	"sync"
	"time"
)

type RouterSessionStatus string

const (
	RouterSessionStatusPending     RouterSessionStatus = "pending"
	RouterSessionStatusEstablished RouterSessionStatus = "established"
	RouterSessionStatusActive      RouterSessionStatus = "active"
	RouterSessionStatusDown        RouterSessionStatus = "down"
)

type LGTargetID struct {
	CollectorID string
	SessionID   string
}

func (id LGTargetID) String() string {
	return id.CollectorID + ":" + id.SessionID
}

type RouterSession struct {
	ID              string              `json:"id"`
	CollectorID     string              `json:"collector_id"`
	DisplayName     string              `json:"display_name"`
	ASN             uint32              `json:"asn"`
	NeighborAddress netip.Addr          `json:"neighbor_address"`
	Status          RouterSessionStatus `json:"status"`
	AfiSafis        []AfiSafi           `json:"afi_safis"`
	LastUpdate      time.Time           `json:"last_update"`
	RouteCount      uint32              `json:"route_count"`
	mu              sync.RWMutex        `json:"-"`
}

// GetStatus returns the session status in a thread-safe manner.
func (rs *RouterSession) GetStatus() RouterSessionStatus {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.Status
}

// SetStatus sets the session status in a thread-safe manner.
func (rs *RouterSession) SetStatus(s RouterSessionStatus) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.Status = s
}

// GetLastUpdate returns the last update timestamp in a thread-safe manner.
func (rs *RouterSession) GetLastUpdate() time.Time {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.LastUpdate
}

// SetLastUpdate sets the last update timestamp in a thread-safe manner.
func (rs *RouterSession) SetLastUpdate(t time.Time) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.LastUpdate = t
}

// GetRouteCount returns the route count in a thread-safe manner.
func (rs *RouterSession) GetRouteCount() uint32 {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.RouteCount
}

// SetRouteCount sets the route count in a thread-safe manner.
func (rs *RouterSession) SetRouteCount(count uint32) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.RouteCount = count
}

type AfiSafi struct {
	Afi  string `json:"afi"`
	Safi string `json:"safi"`
}

func (rs *RouterSession) LGTargetID() LGTargetID {
	return LGTargetID{
		CollectorID: rs.CollectorID,
		SessionID:   rs.ID,
	}
}
