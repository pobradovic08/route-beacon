package store

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pobradovic08/route-beacon/internal/model"
)

// ListRouters returns all known routers with their online/offline status.
// Routers are discovered from both the routers table (BMP initiation) and
// rib_sync_status (active sessions), so routers appear even before the
// ingester has processed the BMP initiation message.
func (db *DB) ListRouters(ctx context.Context) ([]model.Router, error) {
	rows, err := db.Pool.Query(ctx, `
		WITH known_routers AS (
			SELECT router_id FROM routers
			UNION
			SELECT DISTINCT router_id FROM rib_sync_status
		)
		SELECT kr.router_id,
		       r.router_ip,
		       r.hostname,
		       r.as_number,
		       r.description,
		       COALESCE(r.first_seen, s_min.session_start) AS first_seen,
		       COALESCE(r.last_seen, s_min.updated) AS last_seen,
		       EXISTS(SELECT 1 FROM rib_sync_status s WHERE s.router_id = kr.router_id) AS is_online,
		       COALESCE(
		         (SELECT bool_and(s.eor_seen) FROM rib_sync_status s WHERE s.router_id = kr.router_id),
		         false
		       ) AS all_eor_received
		FROM known_routers kr
		LEFT JOIN routers r ON r.router_id = kr.router_id
		LEFT JOIN LATERAL (
			SELECT MIN(session_start_time) AS session_start,
			       MAX(updated_at) AS updated
			FROM rib_sync_status s
			WHERE s.router_id = kr.router_id
		) s_min ON true
		ORDER BY kr.router_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routers []model.Router
	for rows.Next() {
		var (
			routerID    string
			routerIP    *net.IP
			hostname    *string
			asNumber    *int64
			description *string
			firstSeen   *time.Time
			lastSeen    *time.Time
			isOnline    bool
			allEOR      bool
		)
		if err := rows.Scan(&routerID, &routerIP, &hostname, &asNumber, &description,
			&firstSeen, &lastSeen, &isOnline, &allEOR); err != nil {
			return nil, err
		}

		displayName := routerID
		if hostname != nil {
			displayName = *hostname
		}

		status := "down"
		if isOnline {
			status = "up"
		}

		var ipStr *string
		if routerIP != nil {
			s := routerIP.String()
			ipStr = &s
		}

		fs := ""
		if firstSeen != nil {
			fs = model.FormatTime(*firstSeen)
		}
		ls := ""
		if lastSeen != nil {
			ls = model.FormatTime(*lastSeen)
		}

		routers = append(routers, model.Router{
			ID:          routerID,
			RouterIP:    ipStr,
			Hostname:    hostname,
			DisplayName: displayName,
			ASNumber:    asNumber,
			Description: description,
			Status:      status,
			EORReceived: allEOR,
			FirstSeen:   fs,
			LastSeen:    ls,
		})
	}
	if routers == nil {
		routers = []model.Router{}
	}
	return routers, rows.Err()
}

// GetRouter returns a single router by ID, or nil if not found.
func (db *DB) GetRouter(ctx context.Context, routerID string) (*model.Router, error) {
	var (
		routerIP    *net.IP
		hostname    *string
		asNumber    *int64
		description *string
		firstSeen   *time.Time
		lastSeen    *time.Time
		isOnline    bool
		allEOR      bool
	)
	err := db.Pool.QueryRow(ctx, `
		SELECT r.router_ip,
		       r.hostname,
		       r.as_number,
		       r.description,
		       COALESCE(r.first_seen, s_min.session_start) AS first_seen,
		       COALESCE(r.last_seen, s_min.updated) AS last_seen,
		       EXISTS(SELECT 1 FROM rib_sync_status s WHERE s.router_id = $1) AS is_online,
		       COALESCE(
		         (SELECT bool_and(s.eor_seen) FROM rib_sync_status s WHERE s.router_id = $1),
		         false
		       ) AS all_eor_received
		FROM (SELECT $1 AS router_id) AS q
		LEFT JOIN routers r ON r.router_id = q.router_id
		LEFT JOIN LATERAL (
			SELECT MIN(session_start_time) AS session_start,
			       MAX(updated_at) AS updated
			FROM rib_sync_status s
			WHERE s.router_id = $1
		) s_min ON true
		WHERE r.router_id IS NOT NULL
		   OR EXISTS(SELECT 1 FROM rib_sync_status s WHERE s.router_id = $1)
	`, routerID).Scan(&routerIP, &hostname, &asNumber, &description,
		&firstSeen, &lastSeen, &isOnline, &allEOR)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	displayName := routerID
	if hostname != nil {
		displayName = *hostname
	}

	status := "down"
	if isOnline {
		status = "up"
	}

	var ipStr *string
	if routerIP != nil {
		s := routerIP.String()
		ipStr = &s
	}

	fs := ""
	if firstSeen != nil {
		fs = model.FormatTime(*firstSeen)
	}
	ls := ""
	if lastSeen != nil {
		ls = model.FormatTime(*lastSeen)
	}

	return &model.Router{
		ID:          routerID,
		RouterIP:    ipStr,
		Hostname:    hostname,
		DisplayName: displayName,
		ASNumber:    asNumber,
		Description: description,
		Status:      status,
		EORReceived: allEOR,
		FirstSeen:   fs,
		LastSeen:    ls,
	}, nil
}

// GetRouterSummary returns minimal router info for embedding in responses.
func (db *DB) GetRouterSummary(ctx context.Context, routerID string) (*model.RouterSummary, string, error) {
	var (
		hostname *string
		asNumber *int64
		isOnline bool
	)
	err := db.Pool.QueryRow(ctx, `
		SELECT r.hostname, r.as_number,
		       EXISTS(SELECT 1 FROM rib_sync_status s WHERE s.router_id = $1) AS is_online
		FROM (SELECT $1 AS router_id) AS q
		LEFT JOIN routers r ON r.router_id = q.router_id
		WHERE r.router_id IS NOT NULL
		   OR EXISTS(SELECT 1 FROM rib_sync_status s WHERE s.router_id = $1)
		   OR EXISTS(SELECT 1 FROM current_routes cr WHERE cr.router_id = $1 LIMIT 1)
		   OR EXISTS(SELECT 1 FROM route_events re WHERE re.router_id = $1 LIMIT 1)
	`, routerID).Scan(&hostname, &asNumber, &isOnline)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", err
	}

	displayName := routerID
	if hostname != nil {
		displayName = *hostname
	}

	status := "down"
	if isOnline {
		status = "up"
	}

	return &model.RouterSummary{
		ID:          routerID,
		DisplayName: displayName,
		ASNumber:    asNumber,
	}, status, nil
}
