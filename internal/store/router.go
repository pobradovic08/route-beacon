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
// Uses the routers_overview view which joins routers, current_routes, and
// rib_sync_status automatically.
func (db *DB) ListRouters(ctx context.Context) ([]model.Router, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT router_id, router_ip, hostname, as_number, description,
		       display_name, location,
		       first_seen, last_seen,
		       route_count > 0 OR session_start_time IS NOT NULL AS is_online,
		       COALESCE(all_afis_synced, false) AS all_eor_received
		FROM routers_overview
		ORDER BY router_id
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
			displayName *string
			location    *string
			firstSeen   *time.Time
			lastSeen    *time.Time
			isOnline    bool
			allEOR      bool
		)
		if err := rows.Scan(&routerID, &routerIP, &hostname, &asNumber, &description,
			&displayName, &location, &firstSeen, &lastSeen, &isOnline, &allEOR); err != nil {
			return nil, err
		}

		name := routerID
		if displayName != nil {
			name = *displayName
		} else if hostname != nil {
			name = *hostname
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
			DisplayName: name,
			ASNumber:    asNumber,
			Description: description,
			Location:    location,
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
		displayName *string
		location    *string
		firstSeen   *time.Time
		lastSeen    *time.Time
		isOnline    bool
		allEOR      bool
	)
	err := db.Pool.QueryRow(ctx, `
		SELECT router_ip, hostname, as_number, description,
		       display_name, location,
		       first_seen, last_seen,
		       route_count > 0 OR session_start_time IS NOT NULL AS is_online,
		       COALESCE(all_afis_synced, false) AS all_eor_received
		FROM routers_overview
		WHERE router_id = $1
	`, routerID).Scan(&routerIP, &hostname, &asNumber, &description,
		&displayName, &location, &firstSeen, &lastSeen, &isOnline, &allEOR)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	name := routerID
	if displayName != nil {
		name = *displayName
	} else if hostname != nil {
		name = *hostname
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
		DisplayName: name,
		ASNumber:    asNumber,
		Description: description,
		Location:    location,
		Status:      status,
		EORReceived: allEOR,
		FirstSeen:   fs,
		LastSeen:    ls,
	}, nil
}

// GetRouterDetail returns a router with routing table statistics.
func (db *DB) GetRouterDetail(ctx context.Context, routerID string) (*model.RouterDetail, error) {
	var (
		routerIP     *net.IP
		hostname     *string
		asNumber     *int64
		description  *string
		displayName  *string
		location     *string
		firstSeen    *time.Time
		lastSeen     *time.Time
		isOnline     bool
		allEOR       bool
		sessionStart  *time.Time
		syncUpdatedAt *time.Time
		routeCount         int64
		avgASPathLen       *float64
		uniquePfx          int64
		peerCount          int64
		ipv4Routes         int64
		ipv6Routes         int64
		adjRibInRouteCount int64
	)
	err := db.Pool.QueryRow(ctx, `
		WITH stats AS (
			SELECT
				COUNT(*) AS route_count,
				COUNT(DISTINCT prefix) AS unique_prefixes,
				COUNT(DISTINCT nexthop) AS peer_count,
				COUNT(*) FILTER (WHERE afi = 4) AS ipv4_routes,
				COUNT(*) FILTER (WHERE afi = 6) AS ipv6_routes,
				AVG(array_length(string_to_array(as_path, ' '), 1))
					FILTER (WHERE as_path IS NOT NULL AND as_path != '') AS avg_as_path_len
			FROM current_routes
			WHERE router_id = $1
		)
		SELECT ro.router_ip, ro.hostname, ro.as_number, ro.description,
		       ro.display_name, ro.location,
		       ro.first_seen, ro.last_seen,
		       ro.route_count > 0 OR ro.session_start_time IS NOT NULL AS is_online,
		       COALESCE(ro.all_afis_synced, false) AS all_eor_received,
		       ro.session_start_time, ro.sync_updated_at,
		       s.route_count, s.unique_prefixes, s.peer_count,
		       s.ipv4_routes, s.ipv6_routes, s.avg_as_path_len,
		       COALESCE(ro.adj_rib_in_route_count, 0)
		FROM routers_overview ro
		CROSS JOIN stats s
		WHERE ro.router_id = $1
	`, routerID).Scan(&routerIP, &hostname, &asNumber, &description,
		&displayName, &location, &firstSeen, &lastSeen,
		&isOnline, &allEOR, &sessionStart, &syncUpdatedAt,
		&routeCount, &uniquePfx, &peerCount, &ipv4Routes, &ipv6Routes, &avgASPathLen,
		&adjRibInRouteCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	name := routerID
	if displayName != nil {
		name = *displayName
	} else if hostname != nil {
		name = *hostname
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

	var ss *string
	if sessionStart != nil {
		v := model.FormatTime(*sessionStart)
		ss = &v
	}

	var sua *string
	if syncUpdatedAt != nil {
		v := model.FormatTime(*syncUpdatedAt)
		sua = &v
	}

	return &model.RouterDetail{
		Router: model.Router{
			ID:          routerID,
			RouterIP:    ipStr,
			Hostname:    hostname,
			DisplayName: name,
			ASNumber:    asNumber,
			Description: description,
			Location:    location,
			Status:      status,
			EORReceived: allEOR,
			FirstSeen:   fs,
			LastSeen:    ls,
		},
		SessionStart:   ss,
		SyncUpdatedAt:  sua,
		RouteCount:     routeCount,
		UniquePrefixes: uniquePfx,
		PeerCount:      peerCount,
		IPv4Routes:     ipv4Routes,
		IPv6Routes:     ipv6Routes,
		AvgASPathLen:       avgASPathLen,
		AdjRibInRouteCount: adjRibInRouteCount,
	}, nil
}

// GetRouterSummary returns minimal router info for embedding in responses.
func (db *DB) GetRouterSummary(ctx context.Context, routerID string) (*model.RouterSummary, string, error) {
	var (
		hostname    *string
		displayName *string
		asNumber    *int64
		isOnline    bool
	)
	err := db.Pool.QueryRow(ctx, `
		SELECT hostname, display_name, as_number,
		       route_count > 0 OR session_start_time IS NOT NULL AS is_online
		FROM routers_overview
		WHERE router_id = $1
	`, routerID).Scan(&hostname, &displayName, &asNumber, &isOnline)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", err
	}

	name := routerID
	if displayName != nil {
		name = *displayName
	} else if hostname != nil {
		name = *hostname
	}

	status := "down"
	if isOnline {
		status = "up"
	}

	return &model.RouterSummary{
		ID:          routerID,
		DisplayName: name,
		ASNumber:    asNumber,
	}, status, nil
}
