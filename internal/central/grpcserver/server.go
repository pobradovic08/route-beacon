package grpcserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	"github.com/pobradovic08/route-beacon/internal/central/registry"
	"github.com/pobradovic08/route-beacon/internal/central/routestore"
	tlsutil "github.com/pobradovic08/route-beacon/internal/shared/tls"
	pb "github.com/pobradovic08/route-beacon/proto/routebeacon/v1"
)

// Server wraps the gRPC server for collector connections.
type Server struct {
	pb.UnimplementedCollectorServiceServer

	grpcServer *grpc.Server
	listenAddr string
	registry   *registry.Registry
	routeStore *routestore.Store
	dispatcher *CommandDispatcher
}

// ServerDeps holds the dependencies for the gRPC server.
type ServerDeps struct {
	ListenAddr string
	CertLoader *tlsutil.CertificateLoader
	CAPath     string
	Registry   *registry.Registry
	RouteStore *routestore.Store
	Dispatcher *CommandDispatcher
}

// NewServer creates a new gRPC server with mTLS and keepalive configuration.
func NewServer(deps ServerDeps) (*Server, error) {
	s := &Server{
		listenAddr: deps.ListenAddr,
		registry:   deps.Registry,
		routeStore: deps.RouteStore,
		dispatcher: deps.Dispatcher,
	}

	var opts []grpc.ServerOption

	// Configure mTLS if cert loader is provided
	if deps.CertLoader != nil {
		caPool, err := tlsutil.LoadCAPool(deps.CAPath)
		if err != nil {
			return nil, fmt.Errorf("load CA pool: %w", err)
		}
		tlsConfig := tlsutil.NewServerTLSConfig(deps.CertLoader, caPool)
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	} else {
		slog.Warn("gRPC server starting WITHOUT TLS — collector identity is not authenticated; do not use in production")
	}

	// Keepalive parameters
	opts = append(opts,
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)

	s.grpcServer = grpc.NewServer(opts...)
	pb.RegisterCollectorServiceServer(s.grpcServer, s)

	return s, nil
}

// Start begins listening for gRPC connections.
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("gRPC listen: %w", err)
	}
	slog.Info("starting gRPC server", "addr", s.listenAddr)
	if err := s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("gRPC serve: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the gRPC server.
func (s *Server) Shutdown(_ context.Context) {
	slog.Info("shutting down gRPC server")
	s.grpcServer.GracefulStop()
}

// SyncRoutes handles the bidirectional SyncRoutes stream.
func (s *Server) SyncRoutes(stream pb.CollectorService_SyncRoutesServer) error {
	return s.syncRoutes(stream)
}

// SubscribeCommands handles the server-streaming SubscribeCommands RPC.
func (s *Server) SubscribeCommands(req *pb.CollectorIdentity, stream pb.CollectorService_SubscribeCommandsServer) error {
	return s.subscribeCommands(req, stream)
}

// ReportCommandResult handles the client-streaming ReportCommandResult RPC.
func (s *Server) ReportCommandResult(stream pb.CollectorService_ReportCommandResultServer) error {
	return s.reportCommandResult(stream)
}
