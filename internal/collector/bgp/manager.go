package bgp

import (
	"context"
	"fmt"
	"log/slog"

	apipb "github.com/osrg/gobgp/v3/api"
	"github.com/osrg/gobgp/v3/pkg/server"
	"google.golang.org/protobuf/types/known/anypb"

	collectorconfig "github.com/pobradovic08/route-beacon/internal/collector/config"
)

// Manager wraps GoBGP's BgpServer for BGP session management.
type Manager struct {
	server *server.BgpServer
	cfg    *collectorconfig.Config
}

// NewManager creates a new BGP manager with an embedded GoBGP server.
func NewManager(cfg *collectorconfig.Config) *Manager {
	s := server.NewBgpServer()
	go s.Serve()

	return &Manager{
		server: s,
		cfg:    cfg,
	}
}

// Start initializes the BGP server and adds all configured peers.
func (m *Manager) Start(ctx context.Context) error {
	// Start BGP with local config
	if err := m.server.StartBgp(ctx, &apipb.StartBgpRequest{
		Global: &apipb.Global{
			Asn:        m.cfg.BGP.LocalASN,
			RouterId:   m.cfg.BGP.RouterID,
			ListenPort: int32(m.cfg.BGP.ListenPort),
		},
	}); err != nil {
		return fmt.Errorf("start BGP: %w", err)
	}

	slog.Info("BGP server started",
		"asn", m.cfg.BGP.LocalASN,
		"router_id", m.cfg.BGP.RouterID,
		"listen_port", m.cfg.BGP.ListenPort,
	)

	// Add peers
	for _, peer := range m.cfg.BGP.Peers {
		if err := m.addPeer(ctx, peer); err != nil {
			slog.Error("failed to add peer", "neighbor", peer.Neighbor, "error", err)
			return err
		}
	}

	return nil
}

func (m *Manager) addPeer(ctx context.Context, peer collectorconfig.PeerConfig) error {
	// Build AFI/SAFI configurations for IPv4 and IPv6 unicast
	afiSafis := []*apipb.AfiSafi{
		{
			Config: &apipb.AfiSafiConfig{
				Family: &apipb.Family{
					Afi:  apipb.Family_AFI_IP,
					Safi: apipb.Family_SAFI_UNICAST,
				},
				Enabled: true,
			},
			AddPaths: &apipb.AddPaths{
				Config: &apipb.AddPathsConfig{
					Receive: peer.AddPathReceive,
				},
			},
		},
		{
			Config: &apipb.AfiSafiConfig{
				Family: &apipb.Family{
					Afi:  apipb.Family_AFI_IP6,
					Safi: apipb.Family_SAFI_UNICAST,
				},
				Enabled: true,
			},
			AddPaths: &apipb.AddPaths{
				Config: &apipb.AddPathsConfig{
					Receive: peer.AddPathReceive,
				},
			},
		},
	}

	// Build peer config
	peerConf := &apipb.Peer{
		Conf: &apipb.PeerConf{
			NeighborAddress: peer.Neighbor,
			PeerAsn:         peer.ASN,
			Description:     peer.DisplayName,
		},
		// RouteServer client preserves original attributes
		RouteServer: &apipb.RouteServer{
			RouteServerClient: true,
		},
		AfiSafis: afiSafis,
		Transport: &apipb.Transport{
			PassiveMode: peer.Passive,
		},
	}

	// Set import policy to REJECT to avoid storing routes in Loc-RIB
	// Routes are only kept in Adj-RIB-In per peer
	peerConf.ApplyPolicy = &apipb.ApplyPolicy{
		InPolicy: &apipb.PolicyAssignment{
			DefaultAction: apipb.RouteAction_REJECT,
		},
	}

	if err := m.server.AddPeer(ctx, &apipb.AddPeerRequest{Peer: peerConf}); err != nil {
		return fmt.Errorf("add peer %s: %w", peer.Neighbor, err)
	}

	slog.Info("added BGP peer",
		"neighbor", peer.Neighbor,
		"asn", peer.ASN,
		"display_name", peer.DisplayName,
		"passive", peer.Passive,
		"addpath", peer.AddPathReceive,
	)

	return nil
}

// ListPeers returns the current status of all BGP peers.
func (m *Manager) ListPeers(ctx context.Context) ([]*apipb.Peer, error) {
	var peers []*apipb.Peer
	err := m.server.ListPeer(ctx, &apipb.ListPeerRequest{}, func(p *apipb.Peer) {
		peers = append(peers, p)
	})
	if err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	return peers, nil
}

// ListPaths returns all paths from Adj-RIB-In for a specific peer and address family.
func (m *Manager) ListPaths(ctx context.Context, neighborAddr string, family *apipb.Family) ([]*apipb.Destination, error) {
	var destinations []*apipb.Destination
	err := m.server.ListPath(ctx, &apipb.ListPathRequest{
		TableType: apipb.TableType_ADJ_IN,
		Name:      neighborAddr,
		Family:    family,
	}, func(d *apipb.Destination) {
		destinations = append(destinations, d)
	})
	if err != nil {
		return nil, fmt.Errorf("list paths: %w", err)
	}
	return destinations, nil
}

// WatchEvent subscribes to BGP events from the server.
// The provided callback is invoked for each event received.
func (m *Manager) WatchEvent(ctx context.Context, fn func(*apipb.WatchEventResponse)) error {
	// Request to watch for best path changes and peer state changes
	watchReq := &apipb.WatchEventRequest{
		Table: &apipb.WatchEventRequest_Table{
			Filters: []*apipb.WatchEventRequest_Table_Filter{
				{
					Type: apipb.WatchEventRequest_Table_Filter_ADJIN,
				},
			},
		},
		Peer: &apipb.WatchEventRequest_Peer{},
	}

	if err := m.server.WatchEvent(ctx, watchReq, fn); err != nil {
		return fmt.Errorf("watch event: %w", err)
	}
	return nil
}

// EstablishedCount returns the number of BGP peers in ESTABLISHED state.
func (m *Manager) EstablishedCount(ctx context.Context) int {
	peers, err := m.ListPeers(ctx)
	if err != nil {
		return 0
	}
	count := 0
	for _, p := range peers {
		if p.State != nil && p.State.SessionState == apipb.PeerState_ESTABLISHED {
			count++
		}
	}
	return count
}

// WatchRoutes starts watching for BGP table changes and returns a channel of
// RouteEvent. The callback is invoked asynchronously by GoBGP's event loop;
// the returned channel is never closed (it lives for the lifetime of ctx).
func (m *Manager) WatchRoutes(ctx context.Context) (<-chan RouteEvent, error) {
	eventCh := make(chan RouteEvent, 4096)

	err := m.WatchEvent(ctx, func(resp *apipb.WatchEventResponse) {
		table := resp.GetTable()
		if table == nil {
			return
		}
		for _, path := range table.Paths {
			prefix, err := ParsePrefixFromPath(path)
			if err != nil {
				continue
			}
			bgpPath, err := ConvertPath(path)
			if err != nil {
				continue
			}
			event := RouteEvent{
				SessionID: path.NeighborIp,
				IsAdd:     !path.IsWithdraw,
				Prefix:    prefix,
				Path:      bgpPath,
				PathID:    path.Identifier,
			}
			select {
			case eventCh <- event:
			default:
				slog.Warn("route event channel full, dropping event")
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("watch routes: %w", err)
	}

	return eventCh, nil
}

// Server returns the underlying GoBGP server for advanced operations.
func (m *Manager) Server() *server.BgpServer {
	return m.server
}

// Stop shuts down the BGP server.
func (m *Manager) Stop(ctx context.Context) error {
	if err := m.server.StopBgp(ctx, &apipb.StopBgpRequest{}); err != nil {
		return fmt.Errorf("stop BGP: %w", err)
	}
	slog.Info("BGP server stopped")
	return nil
}

// ParsePathAttributes is a helper to extract anyPb attributes from a GoBGP path.
// It's used by the watcher to convert GoBGP paths to the internal model.
func ParsePathAttributes(pattrs []*anypb.Any) map[string]*anypb.Any {
	result := make(map[string]*anypb.Any, len(pattrs))
	for _, a := range pattrs {
		result[string(a.TypeUrl)] = a
	}
	return result
}
