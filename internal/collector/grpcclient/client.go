package grpcclient

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	collectorconfig "github.com/pobradovic08/route-beacon/internal/collector/config"
	tlsutil "github.com/pobradovic08/route-beacon/internal/shared/tls"
	pb "github.com/pobradovic08/route-beacon/proto/routebeacon/v1"
)

// Client manages the gRPC connection from collector to central.
type Client struct {
	conn       *grpc.ClientConn
	client     pb.CollectorServiceClient
	certLoader *tlsutil.CertificateLoader
	cfg        *collectorconfig.Config
}

// New creates a new gRPC client and connects to the central server.
func New(cfg *collectorconfig.Config) (*Client, error) {
	c := &Client{cfg: cfg}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) connect() error {
	// Close any existing connection to prevent leaking on reconnect.
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.client = nil
	}
	// Close any existing certificate loader to prevent leaking watchers/goroutines.
	if c.certLoader != nil {
		c.certLoader.Close()
		c.certLoader = nil
	}

	var opts []grpc.DialOption

	// Configure TLS
	if c.cfg.TLS.Cert != "" && c.cfg.TLS.Key != "" && c.cfg.TLS.CA != "" {
		certLoader, err := tlsutil.NewCertificateLoader(c.cfg.TLS.Cert, c.cfg.TLS.Key)
		if err != nil {
			return fmt.Errorf("load client certificate: %w", err)
		}
		c.certLoader = certLoader
		caPool, err := tlsutil.LoadCAPool(c.cfg.TLS.CA)
		if err != nil {
			c.certLoader.Close()
			c.certLoader = nil
			return fmt.Errorf("load CA: %w", err)
		}
		tlsConfig := tlsutil.NewClientTLSConfig(certLoader, caPool, "")
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		slog.Warn("connecting to central WITHOUT TLS — connection is not authenticated; do not use in production")
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Keepalive
	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:                30 * time.Second,
		Timeout:             10 * time.Second,
		PermitWithoutStream: true,
	}))

	conn, err := grpc.NewClient(c.cfg.Central.Address, opts...)
	if err != nil {
		return fmt.Errorf("connect to central: %w", err)
	}

	c.conn = conn
	c.client = pb.NewCollectorServiceClient(conn)

	slog.Info("connected to central", "addr", c.cfg.Central.Address)
	return nil
}

// Register sends the Register RPC to central.
func (c *Client) Register(ctx context.Context) error {
	sessions := make([]*pb.RouterSession, 0, len(c.cfg.BGP.Peers))
	for _, peer := range c.cfg.BGP.Peers {
		sessions = append(sessions, &pb.RouterSession{
			SessionId:       peer.Neighbor,
			DisplayName:     peer.DisplayName,
			Asn:             peer.ASN,
			NeighborAddress: peer.Neighbor,
		})
	}

	resp, err := c.client.Register(ctx, &pb.RegisterRequest{
		CollectorId:    c.cfg.Collector.ID,
		LocationName:   c.cfg.Collector.Location,
		RouterSessions: sessions,
	})
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	if !resp.Accepted {
		return fmt.Errorf("registration rejected: %s", resp.Message)
	}

	slog.Info("registered with central",
		"collector_id", c.cfg.Collector.ID,
		"sessions", len(sessions),
	)
	return nil
}

// ServiceClient returns the raw gRPC service client for stream operations.
func (c *Client) ServiceClient() pb.CollectorServiceClient {
	return c.client
}

// Close closes the gRPC connection and releases associated resources.
func (c *Client) Close() error {
	if c.certLoader != nil {
		c.certLoader.Close()
		c.certLoader = nil
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ReconnectLoop attempts to reconnect to central with exponential backoff.
// It calls the provided onConnect callback after each successful reconnection.
func (c *Client) ReconnectLoop(ctx context.Context, onConnect func(context.Context) error) {
	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		delay := backoffDelay(attempt)
		slog.Info("reconnecting to central", "attempt", attempt+1, "delay", delay)

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		if err := c.connect(); err != nil {
			slog.Error("reconnect failed", "error", err)
			attempt++
			continue
		}

		if err := c.Register(ctx); err != nil {
			slog.Error("re-registration failed", "error", err)
			attempt++
			continue
		}

		if onConnect != nil {
			if err := onConnect(ctx); err != nil {
				slog.Error("post-connect callback failed", "error", err)
				attempt++
				continue
			}
		}

		// Reset on success
		attempt = 0
		return
	}
}

func backoffDelay(attempt int) time.Duration {
	base := time.Second
	max := 60 * time.Second
	delay := time.Duration(float64(base) * math.Pow(2, float64(attempt)))
	if delay > max {
		delay = max
	}
	return delay
}
