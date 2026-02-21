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
	Status      string  `json:"status"`
	EORReceived bool    `json:"eor_received"`
	FirstSeen   string  `json:"first_seen"`
	LastSeen    string  `json:"last_seen"`
}

// RouterListResponse wraps a list of routers.
type RouterListResponse struct {
	Data []Router `json:"data"`
}

// FormatTime formats a time.Time to ISO 8601.
func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
