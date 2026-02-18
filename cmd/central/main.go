package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pobradovic08/route-beacon/internal/central/api"
	"github.com/pobradovic08/route-beacon/internal/central/config"
	"github.com/pobradovic08/route-beacon/internal/central/grpcserver"
	"github.com/pobradovic08/route-beacon/internal/central/ratelimit"
	"github.com/pobradovic08/route-beacon/internal/central/registry"
	"github.com/pobradovic08/route-beacon/internal/central/routestore"
	tlsutil "github.com/pobradovic08/route-beacon/internal/shared/tls"
)

const shutdownTimeout = 30 * time.Second

func main() {
	configPath := flag.String("config", "configs/central.example.yaml", "path to config file")
	flag.Parse()

	// Configure structured logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("starting central server")

	// Create core stores
	reg := registry.New()
	store := routestore.New()

	// Create rate limiter
	rateLimiter, err := ratelimit.New(
		cfg.RateLimit.RequestsPerInterval,
		cfg.RateLimit.Interval,
		cfg.RateLimit.CleanupInterval,
		cfg.RateLimit.StaleAfter,
	)
	if err != nil {
		slog.Error("failed to create rate limiter", "error", err)
		os.Exit(1)
	}
	defer rateLimiter.Close()

	// Set up TLS (optional for dev)
	var certLoader *tlsutil.CertificateLoader
	if cfg.TLS.Cert != "" && cfg.TLS.Key != "" {
		certLoader, err = tlsutil.NewCertificateLoader(cfg.TLS.Cert, cfg.TLS.Key)
		if err != nil {
			slog.Error("failed to load TLS certificate", "error", err)
			os.Exit(1)
		}
		defer certLoader.Close()
	}

	// Create command dispatcher
	dispatcher := grpcserver.NewCommandDispatcher()

	// Create API handler (implements ServerInterface)
	apiHandler := &api.APIHandler{
		Registry:   reg,
		RouteStore: store,
		StartedAt:  time.Now().Unix(),
		Dispatcher: dispatcher,
	}

	// Create and start API server
	apiServer := api.NewServer(api.ServerDeps{
		Handler:      apiHandler,
		RateLimiter:  rateLimiter,
		ListenAddr:   cfg.API.ListenAddr,
		WriteTimeout: cfg.API.WriteTimeout,
		ReadTimeout:  cfg.API.ReadTimeout,
	})

	// Create and start gRPC server
	grpcSrv, err := grpcserver.NewServer(grpcserver.ServerDeps{
		ListenAddr: cfg.GRPC.ListenAddr,
		CertLoader: certLoader,
		CAPath:     cfg.TLS.CA,
		Registry:   reg,
		RouteStore: store,
		Dispatcher: dispatcher,
	})
	if err != nil {
		slog.Error("failed to create gRPC server", "error", err)
		os.Exit(1)
	}

	// Set up context that gets cancelled on shutdown signal
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Start servers in goroutines
	errCh := make(chan error, 2)

	go func() {
		errCh <- apiServer.Start()
	}()

	go func() {
		errCh <- grpcSrv.Start()
	}()

	slog.Info("central server running",
		"api_addr", cfg.API.ListenAddr,
		"grpc_addr", cfg.GRPC.ListenAddr,
	)

	// Wait for shutdown signal or server error
	select {
	case sig := <-sigCh:
		slog.Info("received shutdown signal", "signal", sig)
	case err := <-errCh:
		slog.Error("server error, initiating shutdown", "error", err)
	}

	// Cancel context to notify all goroutines of shutdown
	cancel()

	// Graceful shutdown with timeout — if the timeout is exceeded, force exit
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	// Force exit goroutine: if the shutdown context expires, force-quit the process
	go func() {
		<-shutdownCtx.Done()
		if shutdownCtx.Err() == context.DeadlineExceeded {
			slog.Error("graceful shutdown timed out, forcing exit")
			os.Exit(1)
		}
	}()

	// Step 1: Stop accepting new HTTP connections, drain in-flight API requests
	slog.Info("shutting down API server, draining in-flight requests")
	if err := apiServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("API server shutdown error", "error", err)
	} else {
		slog.Info("API server stopped")
	}

	// Step 2: Gracefully stop gRPC server (finishes in-flight streams)
	slog.Info("shutting down gRPC server, draining in-flight streams")
	grpcSrv.Shutdown(shutdownCtx)
	slog.Info("gRPC server stopped")

	// Step 3: Save a final route snapshot if snapshot persistence is configured
	if cfg.Snapshots.Enabled {
		slog.Info("saving final route snapshot before exit",
			"directory", cfg.Snapshots.Directory,
			"total_routes", store.TotalRoutes(),
		)
		// TODO: persist snapshot to cfg.Snapshots.Directory once snapshot
		// serialization is implemented (snapshot persistence task).
		// For now we log the intent so operators know the hook is in place.
		slog.Info("final snapshot save completed (persistence not yet implemented)")
	}

	// Step 4: Close rate limiter (deferred) and TLS reloader (deferred)
	slog.Info("central server stopped gracefully")
}
