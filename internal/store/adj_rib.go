package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/pobradovic08/route-beacon/internal/model"
)

// AdjRibInExactLookup returns Adj-RIB-In routes matching the exact prefix
// for a router, filtered by policy type.
func (db *DB) AdjRibInExactLookup(ctx context.Context, routerID, prefix, policy string) ([]model.AdjRibInRoute, error) {
	query := `
		SELECT prefix::text, path_id, is_post_policy, nexthop, as_path, origin,
		       localpref, med, origin_asn,
		       communities_std, communities_ext, communities_large,
		       attrs, first_seen, updated_at
		FROM adj_rib_in
		WHERE router_id = $1
		  AND prefix = $2::cidr`

	args := []any{routerID, prefix}
	query += policyFilter(policy, &args)
	query += `
		ORDER BY is_post_policy, path_id`

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAdjRibInRoutes(rows)
}

// AdjRibInLPMLookup returns Adj-RIB-In routes matching the longest prefix
// for a bare IP address on a specific router.
func (db *DB) AdjRibInLPMLookup(ctx context.Context, routerID, ip, policy string) ([]model.AdjRibInRoute, error) {
	query := `
		WITH lpm AS (
			SELECT prefix
			FROM adj_rib_in
			WHERE router_id = $1
			  AND prefix >>= $2::inet`

	args := []any{routerID, ip}
	query += policyFilter(policy, &args)
	query += `
			ORDER BY masklen(prefix) DESC
			LIMIT 1
		)
		SELECT ar.prefix::text, ar.path_id, ar.is_post_policy, ar.nexthop, ar.as_path, ar.origin,
		       ar.localpref, ar.med, ar.origin_asn,
		       ar.communities_std, ar.communities_ext, ar.communities_large,
		       ar.attrs, ar.first_seen, ar.updated_at
		FROM adj_rib_in ar
		JOIN lpm ON ar.prefix = lpm.prefix
		WHERE ar.router_id = $1`

	query += policyFilter(policy, nil) // reuse same param number
	query += `
		ORDER BY ar.is_post_policy, ar.path_id`

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAdjRibInRoutes(rows)
}

// policyFilter appends a WHERE clause for the is_post_policy filter.
// If args is non-nil, it appends a new parameter. If nil, it reuses the
// last parameter number that was already added.
func policyFilter(policy string, args *[]any) string {
	switch policy {
	case "pre":
		if args != nil {
			*args = append(*args, false)
			return fmt.Sprintf("\n		  AND is_post_policy = $%d", len(*args))
		}
		return "\n		  AND ar.is_post_policy = false"
	case "post":
		if args != nil {
			*args = append(*args, true)
			return fmt.Sprintf("\n		  AND is_post_policy = $%d", len(*args))
		}
		return "\n		  AND ar.is_post_policy = true"
	default: // "both"
		return ""
	}
}

// scanAdjRibInRoutes reads Adj-RIB-In route rows into model slices.
func scanAdjRibInRoutes(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]model.AdjRibInRoute, error) {
	var routes []model.AdjRibInRoute
	for rows.Next() {
		var (
			prefix       string
			pathID       int64
			isPostPolicy bool
			nexthop      *net.IP
			asPathStr    *string
			origin       *string
			localpref    *int
			med          *int
			originASN    *int
			commStd      []string
			commExt      []string
			commLarge    []string
			attrs        json.RawMessage
			firstSeen    time.Time
			updatedAt    time.Time
		)
		if err := rows.Scan(&prefix, &pathID, &isPostPolicy, &nexthop, &asPathStr, &origin,
			&localpref, &med, &originASN,
			&commStd, &commExt, &commLarge,
			&attrs, &firstSeen, &updatedAt); err != nil {
			return nil, err
		}

		var nhStr *string
		if nexthop != nil {
			s := nexthop.String()
			nhStr = &s
		}

		var originLower *string
		if origin != nil {
			l := strings.ToLower(*origin)
			originLower = &l
		}

		if attrs != nil && string(attrs) == "null" {
			attrs = nil
		}

		routes = append(routes, model.AdjRibInRoute{
			Route: model.Route{
				Prefix:              prefix,
				PathID:              pathID,
				NextHop:             nhStr,
				ASPath:              parseASPath(asPathStr),
				Origin:              originLower,
				LocalPref:           localpref,
				MED:                 med,
				OriginASN:           originASN,
				Communities:         parseCommunities(commStd, "standard"),
				ExtendedCommunities: parseCommunities(commExt, "extended"),
				LargeCommunities:    parseCommunities(commLarge, "large"),
				Attrs:               attrs,
				FirstSeen:           model.FormatTime(firstSeen),
				UpdatedAt:           model.FormatTime(updatedAt),
			},
			IsPostPolicy: isPostPolicy,
		})
	}
	if routes == nil {
		routes = []model.AdjRibInRoute{}
	}
	return routes, rows.Err()
}

// GroupAdjRibInPaths groups flat Adj-RIB-In routes by path_id into a nested
// structure with optional pre- and post-policy attribute objects.
func GroupAdjRibInPaths(routes []model.AdjRibInRoute) []model.AdjRibInPath {
	type pathIndex struct {
		idx    int
		pathID int64
	}
	var order []pathIndex
	seen := map[int64]int{} // path_id → index in order

	paths := []model.AdjRibInPath{}
	for _, r := range routes {
		attrs := &model.AdjRibInRouteAttrs{
			NextHop:             r.NextHop,
			ASPath:              r.ASPath,
			Origin:              r.Origin,
			LocalPref:           r.LocalPref,
			MED:                 r.MED,
			OriginASN:           r.OriginASN,
			Communities:         r.Communities,
			ExtendedCommunities: r.ExtendedCommunities,
			LargeCommunities:    r.LargeCommunities,
			Attrs:               r.Attrs,
			FirstSeen:           r.FirstSeen,
			UpdatedAt:           r.UpdatedAt,
		}

		idx, exists := seen[r.PathID]
		if !exists {
			idx = len(paths)
			seen[r.PathID] = idx
			order = append(order, pathIndex{idx: idx, pathID: r.PathID})
			paths = append(paths, model.AdjRibInPath{PathID: r.PathID})
		}

		if r.IsPostPolicy {
			paths[idx].PostPolicy = attrs
		} else {
			paths[idx].PrePolicy = attrs
		}
	}
	return paths
}

// GenerateAdjRibInPlainText produces a text rendering of Adj-RIB-In lookup results.
func GenerateAdjRibInPlainText(prefix, routerID string, paths []model.AdjRibInPath) string {
	if len(paths) == 0 {
		return fmt.Sprintf("No Adj-RIB-In routes found for %s on %s\n", prefix, routerID)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "BGP Adj-RIB-In entry for %s on %s\n", prefix, routerID)
	fmt.Fprintf(&b, "%d path(s) available\n\n", len(paths))

	for i, p := range paths {
		fmt.Fprintf(&b, "Path #%d (path_id %d)", i+1, p.PathID)
		if p.PathID == 0 {
			b.WriteString(" (best)")
		}
		b.WriteString("\n")

		writeAttrs := func(label string, a *model.AdjRibInRouteAttrs) {
			if a == nil {
				return
			}
			fmt.Fprintf(&b, "  [%s]\n", label)
			if a.NextHop != nil {
				fmt.Fprintf(&b, "    Next Hop: %s\n", *a.NextHop)
			}
			fmt.Fprintf(&b, "    AS Path: %s\n", formatASPath(a.ASPath))
			if a.Origin != nil {
				fmt.Fprintf(&b, "    Origin: %s\n", *a.Origin)
			}
			if a.LocalPref != nil {
				fmt.Fprintf(&b, "    Local Pref: %d\n", *a.LocalPref)
			}
			if a.MED != nil {
				fmt.Fprintf(&b, "    MED: %d\n", *a.MED)
			}
			if len(a.Communities) > 0 {
				vals := make([]string, len(a.Communities))
				for j, c := range a.Communities {
					vals[j] = c.Value
				}
				fmt.Fprintf(&b, "    Communities: %s\n", strings.Join(vals, " "))
			}
		}

		writeAttrs("pre-policy", p.PrePolicy)
		writeAttrs("post-policy", p.PostPolicy)
		b.WriteString("\n")
	}
	return b.String()
}

// formatASPath renders an AS path slice as a human-readable string.
func formatASPath(asPath []any) string {
	parts := make([]string, len(asPath))
	for i, a := range asPath {
		switch v := a.(type) {
		case int:
			parts[i] = strconv.Itoa(v)
		case float64:
			parts[i] = strconv.Itoa(int(v))
		case []any:
			nums := make([]string, len(v))
			for k, n := range v {
				switch nn := n.(type) {
				case int:
					nums[k] = strconv.Itoa(nn)
				case float64:
					nums[k] = strconv.Itoa(int(nn))
				default:
					nums[k] = fmt.Sprintf("%v", nn)
				}
			}
			parts[i] = "{" + strings.Join(nums, ",") + "}"
		default:
			parts[i] = fmt.Sprintf("%v", v)
		}
	}
	return strings.Join(parts, " ")
}

// GetAdjRibInHistory returns historical route events for a prefix.
func (db *DB) GetAdjRibInHistory(ctx context.Context, routerID, prefix string, from, to time.Time, limit int) ([]model.AdjRibInRouteEvent, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT ingest_time, action, prefix::text, path_id, COALESCE(is_post_policy, false),
		       nexthop, as_path, origin, localpref, med, origin_asn,
		       communities_std, communities_ext, communities_large
		FROM route_events
		WHERE router_id = $1
		  AND prefix = $2::cidr
		  AND ingest_time BETWEEN $3 AND $4
		ORDER BY ingest_time DESC
		LIMIT $5
	`, routerID, prefix, from, to, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []model.AdjRibInRouteEvent
	for rows.Next() {
		var (
			ingestTime   time.Time
			action       string
			pfx          string
			pathID       *int64
			isPostPolicy bool
			nexthop      *net.IP
			asPathStr    *string
			origin       *string
			localpref    *int
			med          *int
			originASN    *int
			commStd      []string
			commExt      []string
			commLarge    []string
		)
		if err := rows.Scan(&ingestTime, &action, &pfx, &pathID, &isPostPolicy,
			&nexthop, &asPathStr, &origin, &localpref, &med, &originASN,
			&commStd, &commExt, &commLarge); err != nil {
			return nil, err
		}

		actionStr := "announce"
		if action == "D" {
			actionStr = "withdraw"
		}

		var nhStr *string
		if nexthop != nil {
			s := nexthop.String()
			nhStr = &s
		}

		var originLower *string
		if origin != nil {
			l := strings.ToLower(*origin)
			originLower = &l
		}

		events = append(events, model.AdjRibInRouteEvent{
			RouteEvent: model.RouteEvent{
				Timestamp:           model.FormatTime(ingestTime),
				Action:              actionStr,
				Prefix:              pfx,
				PathID:              pathID,
				NextHop:             nhStr,
				ASPath:              parseASPath(asPathStr),
				Origin:              originLower,
				LocalPref:           localpref,
				MED:                 med,
				OriginASN:           originASN,
				Communities:         parseCommunities(commStd, "standard"),
				ExtendedCommunities: parseCommunities(commExt, "extended"),
				LargeCommunities:    parseCommunities(commLarge, "large"),
			},
			IsPostPolicy: isPostPolicy,
		})
	}
	if events == nil {
		events = []model.AdjRibInRouteEvent{}
	}
	return events, rows.Err()
}
