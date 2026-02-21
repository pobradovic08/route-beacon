package model

// HealthResponse represents the system health check response.
type HealthResponse struct {
	Status        string `json:"status"`
	RouterCount   int    `json:"router_count"`
	OnlineRouters int    `json:"online_routers"`
	TotalRoutes   int64  `json:"total_routes"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}
