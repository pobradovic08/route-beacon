package model

import "time"

// Router represents a monitored BGP router.
type Router struct {
	ID          string  `json:"id"`
	RouterIP    *string `json:"router_ip"`
	Hostname    *string `json:"hostname"`
	DisplayName string  `json:"display_name"`
	ASNumber    *int64  `json:"as_number"`
	Description *string `json:"description"`
	Location    *string `json:"location"`
	Status      string  `json:"status"`
	EORReceived bool    `json:"eor_received"`
	FirstSeen   string  `json:"first_seen"`
	LastSeen    string  `json:"last_seen"`
}

// RouterDetail extends Router with routing table statistics.
type RouterDetail struct {
	Router
	SessionStart  *string `json:"session_start"`
	SyncUpdatedAt *string `json:"sync_updated_at"`
	RouteCount    int64   `json:"route_count"`
	UniquePrefixes int64  `json:"unique_prefixes"`
	PeerCount     int64    `json:"peer_count"`
	IPv4Routes    int64    `json:"ipv4_routes"`
	IPv6Routes    int64    `json:"ipv6_routes"`
	AvgASPathLen  *float64 `json:"avg_as_path_len"`
}

// RouterListResponse wraps a list of routers.
type RouterListResponse struct {
	Data []Router `json:"data"`
}

// FormatTime formats a time.Time to ISO 8601.
func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
