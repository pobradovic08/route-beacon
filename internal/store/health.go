package store

import "context"

// HealthSummary holds the raw data for health status derivation.
type HealthSummary struct {
	RouterCount   int
	OnlineRouters int
	TotalRoutes   int64
	AllEOR        *bool // nil when no sync rows exist
}

// GetHealthSummary queries aggregate health stats from the database.
func (db *DB) GetHealthSummary(ctx context.Context) (*HealthSummary, error) {
	var s HealthSummary
	err := db.Pool.QueryRow(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM (
		    SELECT router_id FROM routers
		    UNION
		    SELECT DISTINCT router_id FROM rib_sync_status
		  ) known) AS router_count,
		  (SELECT COUNT(DISTINCT router_id) FROM rib_sync_status) AS online_routers,
		  COALESCE(
		    NULLIF((SELECT SUM(route_count) FROM route_summary), 0),
		    (SELECT COUNT(*) FROM current_routes)
		  ) AS total_routes,
		  (SELECT bool_and(eor_seen) FROM rib_sync_status) AS all_eor
	`).Scan(&s.RouterCount, &s.OnlineRouters, &s.TotalRoutes, &s.AllEOR)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
