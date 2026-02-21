package store

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/pobradovic08/route-beacon/internal/model"
)

// GetRouteHistory returns historical route events for a prefix on a router.
func (db *DB) GetRouteHistory(ctx context.Context, routerID, prefix string, from, to time.Time, limit int) ([]model.RouteEvent, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT ingest_time, action, prefix::text, path_id, nexthop, as_path,
		       origin, localpref, med, origin_asn,
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

	var events []model.RouteEvent
	for rows.Next() {
		var (
			ingestTime time.Time
			action     string
			pfx        string
			pathID     *int64
			nexthop    *net.IP
			asPathStr  *string
			origin     *string
			localpref  *int
			med        *int
			originASN  *int
			commStd    []string
			commExt    []string
			commLarge  []string
		)
		if err := rows.Scan(&ingestTime, &action, &pfx, &pathID, &nexthop, &asPathStr,
			&origin, &localpref, &med, &originASN,
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

		events = append(events, model.RouteEvent{
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
		})
	}
	if events == nil {
		events = []model.RouteEvent{}
	}
	return events, rows.Err()
}
