package grpcserver

import (
	"context"
	"log/slog"
	"net/netip"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/pobradovic08/route-beacon/internal/shared/model"
	tlsutil "github.com/pobradovic08/route-beacon/internal/shared/tls"
	pb "github.com/pobradovic08/route-beacon/proto/routebeacon/v1"
)

// Register handles the Register RPC from a collector.
func (s *Server) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	collectorID := req.CollectorId

	// Try to extract collector ID from mTLS peer cert CN.
	if p, ok := peer.FromContext(ctx); ok && p.AuthInfo != nil {
		if tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			if cn, err := tlsutil.ExtractCollectorID(tlsInfo.State); err == nil {
				collectorID = cn
			}
		}
	}

	if collectorID == "" {
		return nil, status.Error(codes.InvalidArgument, "collector_id is required")
	}

	slog.Info("collector registering",
		"collector_id", collectorID,
		"location", req.LocationName,
		"sessions", len(req.RouterSessions),
	)

	// Convert proto sessions to model sessions.
	sessions := make([]*model.RouterSession, 0, len(req.RouterSessions))
	for _, ps := range req.RouterSessions {
		sess := &model.RouterSession{
			ID:          ps.SessionId,
			DisplayName: ps.DisplayName,
			ASN:         ps.Asn,
			Status:      model.RouterSessionStatusPending,
		}

		// Parse neighbor address from string.
		if ps.NeighborAddress != "" {
			addr, err := netip.ParseAddr(ps.NeighborAddress)
			if err != nil {
				slog.Warn("invalid neighbor address", "address", ps.NeighborAddress, "error", err)
			} else {
				sess.NeighborAddress = addr
			}
		}

		sessions = append(sessions, sess)
	}

	// Register in the registry.
	if err := s.registry.RegisterCollector(collectorID, req.LocationName, sessions); err != nil {
		slog.Error("failed to register collector", "collector_id", collectorID, "error", err)
		return &pb.RegisterResponse{
			Accepted: false,
			Message:  err.Error(),
		}, nil
	}

	slog.Info("collector connected",
		"collector_id", collectorID,
		"location", req.LocationName,
		"session_count", len(sessions),
	)

	for _, sess := range sessions {
		slog.Info("session registered",
			"collector_id", collectorID,
			"session_id", sess.ID,
			"display_name", sess.DisplayName,
			"asn", sess.ASN,
			"status", sess.GetStatus(),
		)
	}

	return &pb.RegisterResponse{
		Accepted: true,
		Message:  "registered",
	}, nil
}
