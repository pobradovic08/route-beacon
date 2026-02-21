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

// ExactLookup returns routes matching the exact prefix for a router.
func (db *DB) ExactLookup(ctx context.Context, routerID, prefix string) ([]model.Route, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT prefix::text, path_id, nexthop, as_path, origin,
		       localpref, med, origin_asn,
		       communities_std, communities_ext, communities_large,
		       attrs, first_seen, updated_at
		FROM current_routes
		WHERE router_id = $1
		  AND prefix = $2::cidr
		ORDER BY path_id
	`, routerID, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoutes(rows)
}

// LPMLookup returns routes matching the longest prefix for a bare IP address.
func (db *DB) LPMLookup(ctx context.Context, routerID, ip string) ([]model.Route, error) {
	rows, err := db.Pool.Query(ctx, `
		WITH lpm AS (
			SELECT prefix
			FROM current_routes
			WHERE router_id = $1
			  AND prefix >>= $2::inet
			ORDER BY masklen(prefix) DESC
			LIMIT 1
		)
		SELECT cr.prefix::text, cr.path_id, cr.nexthop, cr.as_path, cr.origin,
		       cr.localpref, cr.med, cr.origin_asn,
		       cr.communities_std, cr.communities_ext, cr.communities_large,
		       cr.attrs, cr.first_seen, cr.updated_at
		FROM current_routes cr
		JOIN lpm ON cr.prefix = lpm.prefix
		WHERE cr.router_id = $1
		ORDER BY cr.path_id
	`, routerID, ip)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoutes(rows)
}

// scanRoutes reads route rows into model.Route slices.
func scanRoutes(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]model.Route, error) {
	var routes []model.Route
	for rows.Next() {
		var (
			prefix         string
			pathID         int64
			nexthop        *net.IP
			asPathStr      *string
			origin         *string
			localpref      *int
			med            *int
			originASN      *int
			commStd        []string
			commExt        []string
			commLarge      []string
			attrs          json.RawMessage
			firstSeen      time.Time
			updatedAt      time.Time
		)
		if err := rows.Scan(&prefix, &pathID, &nexthop, &asPathStr, &origin,
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

		routes = append(routes, model.Route{
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
		})
	}
	if routes == nil {
		routes = []model.Route{}
	}
	return routes, rows.Err()
}

// parseASPath converts a space-delimited AS path string into []any.
// AS_SET segments like {64496,65001} are represented as []any containing ints.
func parseASPath(s *string) []any {
	if s == nil || *s == "" {
		return []any{}
	}

	var result []any
	text := *s
	i := 0
	for i < len(text) {
		if text[i] == ' ' {
			i++
			continue
		}
		if text[i] == '{' {
			// AS_SET segment
			end := strings.IndexByte(text[i:], '}')
			if end < 0 {
				break
			}
			inner := text[i+1 : i+end]
			var set []any
			for _, part := range strings.Split(inner, ",") {
				part = strings.TrimSpace(part)
				if n, err := strconv.Atoi(part); err == nil {
					set = append(set, n)
				}
			}
			result = append(result, set)
			i += end + 1
		} else {
			// Regular AS number
			j := i
			for j < len(text) && text[j] != ' ' && text[j] != '{' {
				j++
			}
			if n, err := strconv.Atoi(text[i:j]); err == nil {
				result = append(result, n)
			}
			i = j
		}
	}
	return result
}

// parseCommunities converts a TEXT[] into Community objects.
func parseCommunities(values []string, commType string) []model.Community {
	if len(values) == 0 {
		return []model.Community{}
	}
	communities := make([]model.Community, len(values))
	for i, v := range values {
		communities[i] = model.Community{Type: commType, Value: v}
	}
	return communities
}

// GeneratePlainText produces a text rendering of route lookup results.
func GeneratePlainText(prefix string, routerID string, routes []model.Route) string {
	if len(routes) == 0 {
		return fmt.Sprintf("No routes found for %s on %s\n", prefix, routerID)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "BGP routing table entry for %s on %s\n", prefix, routerID)
	fmt.Fprintf(&b, "%d path(s) available\n\n", len(routes))

	for i, r := range routes {
		fmt.Fprintf(&b, "Path #%d", i+1)
		if r.PathID == 0 {
			b.WriteString(" (best)")
		}
		b.WriteString("\n")

		if r.NextHop != nil {
			fmt.Fprintf(&b, "  Next Hop: %s\n", *r.NextHop)
		}

		asPathParts := make([]string, len(r.ASPath))
		for j, a := range r.ASPath {
			switch v := a.(type) {
			case int:
				asPathParts[j] = strconv.Itoa(v)
			case float64:
				asPathParts[j] = strconv.Itoa(int(v))
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
				asPathParts[j] = "{" + strings.Join(nums, ",") + "}"
			default:
				asPathParts[j] = fmt.Sprintf("%v", v)
			}
		}
		fmt.Fprintf(&b, "  AS Path: %s\n", strings.Join(asPathParts, " "))

		if r.Origin != nil {
			fmt.Fprintf(&b, "  Origin: %s\n", *r.Origin)
		}
		if r.LocalPref != nil {
			fmt.Fprintf(&b, "  Local Pref: %d\n", *r.LocalPref)
		}
		if r.MED != nil {
			fmt.Fprintf(&b, "  MED: %d\n", *r.MED)
		}
		if len(r.Communities) > 0 {
			vals := make([]string, len(r.Communities))
			for j, c := range r.Communities {
				vals[j] = c.Value
			}
			fmt.Fprintf(&b, "  Communities: %s\n", strings.Join(vals, " "))
		}
		b.WriteString("\n")
	}
	return b.String()
}
