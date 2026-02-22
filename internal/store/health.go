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
		  (SELECT COUNT(*) FROM routers_overview) AS router_count,
		  (SELECT COUNT(*) FROM routers_overview
		   WHERE route_count > 0 OR session_start_time IS NOT NULL) AS online_routers,
		  (SELECT COALESCE(SUM(route_count), 0) FROM routers_overview) AS total_routes,
		  (SELECT bool_and(all_afis_synced) FROM routers_overview
		   WHERE all_afis_synced IS NOT NULL) AS all_eor
	`).Scan(&s.RouterCount, &s.OnlineRouters, &s.TotalRoutes, &s.AllEOR)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
