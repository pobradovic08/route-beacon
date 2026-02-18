package grpcserver

import (
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"time"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
	tlsutil "github.com/pobradovic08/route-beacon/internal/shared/tls"
	pb "github.com/pobradovic08/route-beacon/proto/routebeacon/v1"
)

// SyncRoutes handles the bidirectional SyncRoutes stream from a collector.
func (s *Server) syncRoutes(stream pb.CollectorService_SyncRoutesServer) error {
	// Try to extract collector ID from mTLS peer certificate.
	var collectorID string
	if p, ok := peer.FromContext(stream.Context()); ok && p.AuthInfo != nil {
		if tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			if cn, err := tlsutil.ExtractCollectorID(tlsInfo.State); err == nil {
				collectorID = cn
			}
		}
	}

	// Track snapshot start times per session for duration measurement
	snapshotStartTimes := make(map[string]time.Time)

	slog.Info("sync stream opened")

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			slog.Info("sync stream closed (EOF)",
				"collector_id", collectorID,
			)
			return nil
		}
		if err != nil {
			slog.Error("sync stream recv error",
				"collector_id", collectorID,
				"error", err,
			)
			return err
		}

		switch p := msg.Payload.(type) {
		case *pb.RouteMessage_BeginSnapshot:
			sessionID := p.BeginSnapshot.RouterSessionId

			// Infer collector ID from the registry if not yet known.
			if collectorID == "" {
				var matchedCollector string
				var matchCount int
				targets := s.registry.ListTargets()
				for _, t := range targets {
					if t.ID == sessionID {
						matchedCollector = t.CollectorID
						matchCount++
					}
				}
				if matchCount == 1 {
					collectorID = matchedCollector
				} else if matchCount > 1 {
					slog.Error("cannot begin snapshot: ambiguous session ID matches multiple collectors",
						"session_id", sessionID,
						"match_count", matchCount,
					)
					continue
				}
			}

			// Guard: refuse to create a trie with an empty collector key.
			if collectorID == "" {
				slog.Error("cannot begin snapshot: collector ID unknown",
					"session_id", sessionID,
				)
				continue
			}

			targetID := model.LGTargetID{
				CollectorID: collectorID,
				SessionID:   sessionID,
			}

			snapshotStartTimes[sessionID] = time.Now()

			slog.Info("sync snapshot started",
				"collector_id", collectorID,
				"session_id", sessionID,
			)

			// Create/reset trie for this target
			s.routeStore.CreateTrie(targetID)

			// Mark session as syncing
			s.registry.UpdateSessionStatus(targetID, model.RouterSessionStatusEstablished)

			slog.Info("session status changed",
				"collector_id", collectorID,
				"session_id", sessionID,
				"status", model.RouterSessionStatusEstablished,
			)

		case *pb.RouteMessage_RouteUpdate:
			sessionID := p.RouteUpdate.RouterSessionId
			if collectorID == "" {
				// Try to figure out collector ID from registry
				var matchedCollector string
				var matchCount int
				targets := s.registry.ListTargets()
				for _, t := range targets {
					if t.ID == sessionID {
						matchedCollector = t.CollectorID
						matchCount++
					}
				}
				if matchCount == 1 {
					collectorID = matchedCollector
				} else if matchCount > 1 {
					slog.Warn("ambiguous session ID in route update, skipping",
						"session_id", sessionID,
						"match_count", matchCount,
					)
				}
			}

			targetID := model.LGTargetID{
				CollectorID: collectorID,
				SessionID:   sessionID,
			}

			for _, route := range p.RouteUpdate.Routes {
				prefix, err := protoToPrefix(route)
				if err != nil {
					slog.Warn("invalid prefix in route update",
						"collector_id", collectorID,
						"session_id", sessionID,
						"error", err,
					)
					continue
				}

				switch p.RouteUpdate.Action {
				case pb.RouteAction_ADD:
					path := protoToBGPPath(route)
					s.routeStore.UpsertRoute(targetID, prefix, path)
				case pb.RouteAction_WITHDRAW:
					s.routeStore.WithdrawRoute(targetID, prefix, route.PathId)
				}
			}

			// Update last update timestamp
			s.registry.UpdateSessionLastUpdate(targetID, time.Now())

		case *pb.RouteMessage_EndSnapshot:
			sessionID := p.EndSnapshot.RouterSessionId
			targetID := model.LGTargetID{
				CollectorID: collectorID,
				SessionID:   sessionID,
			}

			actualCount := s.routeStore.Count(targetID)

			// Calculate snapshot duration
			var duration time.Duration
			if startTime, ok := snapshotStartTimes[sessionID]; ok {
				duration = time.Since(startTime)
				delete(snapshotStartTimes, sessionID)
			}

			slog.Info("sync snapshot completed",
				"collector_id", collectorID,
				"session_id", sessionID,
				"expected_route_count", p.EndSnapshot.RouteCount,
				"actual_route_count", actualCount,
				"duration", duration,
			)

			// Mark session as active
			s.registry.UpdateSessionStatus(targetID, model.RouterSessionStatusActive)
			s.registry.UpdateSessionRouteCount(targetID, actualCount)

			slog.Info("session status changed",
				"collector_id", collectorID,
				"session_id", sessionID,
				"status", model.RouterSessionStatusActive,
				"route_count", actualCount,
			)

			// Send ack
			if err := stream.Send(&pb.SyncControl{
				Payload: &pb.SyncControl_SnapshotAck{
					SnapshotAck: &pb.SnapshotAck{
						RouterSessionId: sessionID,
						RoutesReceived:  actualCount,
					},
				},
			}); err != nil {
				slog.Error("failed to send snapshot ack",
					"collector_id", collectorID,
					"session_id", sessionID,
					"error", err,
				)
			}
		}
	}
}

func protoToPrefix(route *pb.BGPRoute) (netip.Prefix, error) {
	var addr netip.Addr
	switch route.Afi {
	case pb.AddressFamily_IPV4:
		if len(route.Prefix) < 4 {
			return netip.Prefix{}, fmt.Errorf("IPv4 prefix too short: %d bytes", len(route.Prefix))
		}
		addr = netip.AddrFrom4([4]byte(route.Prefix[:4]))
	case pb.AddressFamily_IPV6:
		if len(route.Prefix) < 16 {
			return netip.Prefix{}, fmt.Errorf("IPv6 prefix too short: %d bytes", len(route.Prefix))
		}
		addr = netip.AddrFrom16([16]byte(route.Prefix[:16]))
	default:
		return netip.Prefix{}, fmt.Errorf("unknown address family: %v", route.Afi)
	}

	return netip.PrefixFrom(addr, int(route.PrefixLength)), nil
}

func protoToBGPPath(route *pb.BGPRoute) model.BGPPath {
	attrs := route.Attributes
	if attrs == nil {
		return model.BGPPath{
			IsBest:     route.IsBest,
			PathID:     route.PathId,
			ReceivedAt: time.Now(),
		}
	}

	path := model.BGPPath{
		IsBest:          route.IsBest,
		PathID:          route.PathId,
		MED:             attrs.Med,
		LocalPref:       attrs.LocalPref,
		AtomicAggregate: attrs.AtomicAggregate,
		ReceivedAt:      time.Now(),
	}

	// Parse next hop
	if len(attrs.NextHop) == 4 {
		path.NextHop = netip.AddrFrom4([4]byte(attrs.NextHop))
	} else if len(attrs.NextHop) == 16 {
		path.NextHop = netip.AddrFrom16([16]byte(attrs.NextHop))
	}

	// Parse origin
	switch attrs.Origin {
	case 0:
		path.Origin = "igp"
	case 1:
		path.Origin = "egp"
	default:
		path.Origin = "incomplete"
	}

	// MED and LocalPref presence - use explicit presence booleans from proto
	// to correctly handle zero values (proto3 defaults uint32 to 0).
	path.MEDPresent = attrs.MedPresent
	path.LocalPrefPresent = attrs.LocalPrefPresent

	// Parse AS path segments
	for _, seg := range attrs.AsPathSegments {
		segType := "sequence"
		if seg.Type == pb.ASPathSegmentType_AS_SET {
			segType = "set"
		}
		path.ASPath = append(path.ASPath, model.ASPathSegment{
			Type: segType,
			ASNs: seg.Numbers,
		})
	}

	// Parse communities
	for _, c := range attrs.Communities {
		path.Communities = append(path.Communities, model.NewCommunity(c))
	}

	// Parse large communities
	for _, lc := range attrs.LargeCommunities {
		path.LargeCommunities = append(path.LargeCommunities, model.LargeCommunity{
			GlobalAdmin: lc.GlobalAdmin,
			LocalData1:  lc.LocalData_1,
			LocalData2:  lc.LocalData_2,
		})
	}

	// Parse aggregator
	if attrs.AggregatorAsn > 0 {
		agg := &model.Aggregator{ASN: attrs.AggregatorAsn}
		if len(attrs.AggregatorAddress) == 4 {
			agg.Address = netip.AddrFrom4([4]byte(attrs.AggregatorAddress))
		} else if len(attrs.AggregatorAddress) == 16 {
			agg.Address = netip.AddrFrom16([16]byte(attrs.AggregatorAddress))
		}
		path.Aggregator = agg
	}

	return path
}
