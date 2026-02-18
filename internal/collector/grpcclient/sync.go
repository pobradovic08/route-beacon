package grpcclient

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"time"

	apipb "github.com/osrg/gobgp/v3/api"

	"github.com/pobradovic08/route-beacon/internal/collector/bgp"
	"github.com/pobradovic08/route-beacon/internal/shared/model"
	pb "github.com/pobradovic08/route-beacon/proto/routebeacon/v1"
)

// SyncRoutes starts the bidirectional SyncRoutes stream. On initial connect,
// it sends full snapshots for each router session, then streams incremental
// updates from the watcher channel.
func (c *Client) SyncRoutes(ctx context.Context, bgpMgr *bgp.Manager, events <-chan bgp.RouteEvent) error {
	stream, err := c.client.SyncRoutes(ctx)
	if err != nil {
		return fmt.Errorf("open SyncRoutes stream: %w", err)
	}

	// Send full snapshots for each configured peer
	for _, peer := range c.cfg.BGP.Peers {
		if err := c.sendSnapshot(ctx, stream, bgpMgr, peer.Neighbor); err != nil {
			slog.Error("snapshot failed", "session", peer.Neighbor, "error", err)
			continue
		}
	}

	// Read acks in background
	go func() {
		for {
			ctrl, err := stream.Recv()
			if err != nil {
				slog.Error("SyncRoutes recv error", "error", err)
				return
			}
			switch p := ctrl.Payload.(type) {
			case *pb.SyncControl_SnapshotAck:
				slog.Info("snapshot acknowledged",
					"session", p.SnapshotAck.RouterSessionId,
					"routes_received", p.SnapshotAck.RoutesReceived,
				)
			case *pb.SyncControl_Throttle:
				slog.Warn("throttle received", "delay_ms", p.Throttle.DelayMs)
				time.Sleep(time.Duration(p.Throttle.DelayMs) * time.Millisecond)
			}
		}
	}()

	// Stream incremental updates from watcher
	for {
		select {
		case <-ctx.Done():
			stream.CloseSend()
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				stream.CloseSend()
				return nil
			}
			if err := c.sendUpdate(stream, event); err != nil {
				return fmt.Errorf("send update: %w", err)
			}
		}
	}
}

func (c *Client) sendSnapshot(ctx context.Context, stream pb.CollectorService_SyncRoutesClient, bgpMgr *bgp.Manager, sessionID string) error {
	neighborAddr := sessionID

	// Send BeginSnapshot
	if err := stream.Send(&pb.RouteMessage{
		Payload: &pb.RouteMessage_BeginSnapshot{
			BeginSnapshot: &pb.BeginSnapshot{
				RouterSessionId: sessionID,
			},
		},
	}); err != nil {
		return fmt.Errorf("send BeginSnapshot: %w", err)
	}

	var routeCount uint32

	// Walk IPv4 and IPv6 Adj-RIB-In
	families := []*apipb.Family{
		{Afi: apipb.Family_AFI_IP, Safi: apipb.Family_SAFI_UNICAST},
		{Afi: apipb.Family_AFI_IP6, Safi: apipb.Family_SAFI_UNICAST},
	}

	for _, family := range families {
		destinations, err := bgpMgr.ListPaths(ctx, neighborAddr, family)
		if err != nil {
			slog.Warn("list paths failed", "session", sessionID, "family", family, "error", err)
			continue
		}

		batch := make([]*pb.BGPRoute, 0, 1000)
		for _, dest := range destinations {
			for _, path := range dest.Paths {
				prefix, err := bgp.ParsePrefixFromPath(path)
				if err != nil {
					continue
				}
				bgpPath, err := bgp.ConvertPath(path)
				if err != nil {
					continue
				}

				protoRoute := modelPathToProto(prefix, bgpPath)
				batch = append(batch, protoRoute)
				routeCount++

				if len(batch) >= 1000 {
					if err := stream.Send(&pb.RouteMessage{
						Payload: &pb.RouteMessage_RouteUpdate{
							RouteUpdate: &pb.RouteUpdate{
								RouterSessionId: sessionID,
								Action:          pb.RouteAction_ADD,
								Routes:          batch,
							},
						},
					}); err != nil {
						return fmt.Errorf("send batch: %w", err)
					}
					batch = batch[:0]
				}
			}
		}

		// Send remaining batch
		if len(batch) > 0 {
			if err := stream.Send(&pb.RouteMessage{
				Payload: &pb.RouteMessage_RouteUpdate{
					RouteUpdate: &pb.RouteUpdate{
						RouterSessionId: sessionID,
						Action:          pb.RouteAction_ADD,
						Routes:          batch,
					},
				},
			}); err != nil {
				return fmt.Errorf("send final batch: %w", err)
			}
		}
	}

	// Send EndSnapshot
	if err := stream.Send(&pb.RouteMessage{
		Payload: &pb.RouteMessage_EndSnapshot{
			EndSnapshot: &pb.EndSnapshot{
				RouterSessionId: sessionID,
				RouteCount:      routeCount,
			},
		},
	}); err != nil {
		return fmt.Errorf("send EndSnapshot: %w", err)
	}

	slog.Info("snapshot sent", "session", sessionID, "routes", routeCount)
	return nil
}

func (c *Client) sendUpdate(stream pb.CollectorService_SyncRoutesClient, event bgp.RouteEvent) error {
	action := pb.RouteAction_ADD
	if !event.IsAdd {
		action = pb.RouteAction_WITHDRAW
	}

	route := modelPathToProto(event.Prefix, event.Path)

	return stream.Send(&pb.RouteMessage{
		Payload: &pb.RouteMessage_RouteUpdate{
			RouteUpdate: &pb.RouteUpdate{
				RouterSessionId: event.SessionID,
				Action:          action,
				Routes:          []*pb.BGPRoute{route},
			},
		},
	})
}

func modelPathToProto(prefix netip.Prefix, path model.BGPPath) *pb.BGPRoute {
	afi := pb.AddressFamily_IPV4
	if prefix.Addr().Is6() {
		afi = pb.AddressFamily_IPV6
	}

	// Serialize prefix address to bytes
	var prefixBytes []byte
	if prefix.Addr().Is4() {
		a4 := prefix.Addr().As4()
		prefixBytes = a4[:]
	} else {
		a16 := prefix.Addr().As16()
		prefixBytes = a16[:]
	}

	// Convert AS path
	var asPathFlat []uint32
	var segments []*pb.ASPathSegment
	for _, seg := range path.ASPath {
		segType := pb.ASPathSegmentType_AS_SEQUENCE
		if seg.Type == "set" {
			segType = pb.ASPathSegmentType_AS_SET
		}
		segments = append(segments, &pb.ASPathSegment{
			Type:    segType,
			Numbers: seg.ASNs,
		})
		asPathFlat = append(asPathFlat, seg.ASNs...)
	}

	// Convert next hop
	var nextHop []byte
	if path.NextHop.Is4() {
		nh4 := path.NextHop.As4()
		nextHop = nh4[:]
	} else {
		nh16 := path.NextHop.As16()
		nextHop = nh16[:]
	}

	// Convert communities
	var communities []uint32
	for _, c := range path.Communities {
		communities = append(communities, c.Value)
	}

	// Convert extended communities to raw bytes
	var extCommunities [][]byte
	for _, ec := range path.ExtendedCommunities {
		raw := make([]byte, 8)
		copy(raw, ec.Raw[:])
		extCommunities = append(extCommunities, raw)
	}

	// Convert large communities
	var largeCommunities []*pb.LargeCommunity
	for _, lc := range path.LargeCommunities {
		largeCommunities = append(largeCommunities, &pb.LargeCommunity{
			GlobalAdmin: lc.GlobalAdmin,
			LocalData_1: lc.LocalData1,
			LocalData_2: lc.LocalData2,
		})
	}

	attrs := &pb.BGPAttributes{
		AsPath:           asPathFlat,
		AsPathSegments:   segments,
		NextHop:          nextHop,
		Origin:           originToUint32(path.Origin),
		Med:              path.MED,
		LocalPref:        path.LocalPref,
		MedPresent:       path.MEDPresent,
		LocalPrefPresent: path.LocalPrefPresent,
		Communities:      communities,
		ExtCommunities:   extCommunities,
		LargeCommunities: largeCommunities,
		AtomicAggregate:  path.AtomicAggregate,
	}

	if path.Aggregator != nil {
		attrs.AggregatorAsn = path.Aggregator.ASN
		if path.Aggregator.Address.Is4() {
			agg4 := path.Aggregator.Address.As4()
			attrs.AggregatorAddress = agg4[:]
		} else {
			agg16 := path.Aggregator.Address.As16()
			attrs.AggregatorAddress = agg16[:]
		}
	}

	return &pb.BGPRoute{
		Prefix:       prefixBytes,
		PrefixLength: uint32(prefix.Bits()),
		Afi:          afi,
		IsBest:       path.IsBest,
		PathId:       path.PathID,
		Attributes:   attrs,
	}
}

func originToUint32(origin string) uint32 {
	switch origin {
	case "igp":
		return 0
	case "egp":
		return 1
	default:
		return 2
	}
}
