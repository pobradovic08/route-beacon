package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pobradovic08/route-beacon/internal/collector/bgp"
	"github.com/pobradovic08/route-beacon/internal/collector/config"
	"github.com/pobradovic08/route-beacon/internal/collector/grpcclient"
)

const (
	shutdownTimeout    = 30 * time.Second
	sessionWaitTimeout = 60 * time.Second
)

func main() {
	configPath := flag.String("config", "configs/collector.example.yaml", "path to config file")
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

	slog.Info("starting collector",
		"id", cfg.Collector.ID,
		"location", cfg.Collector.Location,
	)

	// Set up context that gets cancelled on shutdown signal.
	// All long-running goroutines (gRPC streams, BGP watchers, command
	// subscribers) should derive from this context so that cancelling it
	// propagates cleanly through the whole process.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Connect to central and register
	grpcClient, err := grpcclient.New(cfg)
	if err != nil {
		slog.Error("failed to connect to central", "error", err)
		os.Exit(1)
	}
	defer grpcClient.Close()

	if err := grpcClient.Register(ctx); err != nil {
		slog.Error("failed to register with central", "error", err)
		os.Exit(1)
	}

	// Initialize GoBGP manager and start peer sessions
	bgpManager := bgp.NewManager(cfg)
	if err := bgpManager.Start(ctx); err != nil {
		slog.Error("failed to start BGP manager", "error", err)
		os.Exit(1)
	}

	// Start watching BGP table changes; the returned channel receives
	// incremental route updates that are forwarded to central.
	eventCh, err := bgpManager.WatchRoutes(ctx)
	if err != nil {
		slog.Error("failed to start BGP route watcher", "error", err)
		os.Exit(1)
	}

	// Wait for at least one BGP session to establish before sending snapshots
	// so that the initial snapshot contains actual routes.
	waitForBGPSessions(ctx, bgpManager, len(cfg.BGP.Peers))

	// Start route sync (snapshots + incremental updates)
	go func() {
		if err := grpcClient.SyncRoutes(ctx, bgpManager, eventCh); err != nil && ctx.Err() == nil {
			slog.Error("SyncRoutes stream error", "error", err)
		}
	}()

	// Start command subscriber (ping/traceroute dispatch)
	go func() {
		if err := grpcClient.SubscribeCommands(ctx); err != nil && ctx.Err() == nil {
			slog.Error("SubscribeCommands stream error", "error", err)
		}
	}()

	slog.Info("collector running", "id", cfg.Collector.ID)

	// Wait for shutdown signal
	sig := <-sigCh
	slog.Info("received shutdown signal", "signal", sig)

	// Cancel the root context — this stops gRPC streams, BGP watchers,
	// command execution, and any other goroutines derived from ctx.
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

	// Step 1: Stop GoBGP manager — tears down BGP sessions cleanly
	slog.Info("stopping BGP manager")
	if err := bgpManager.Stop(shutdownCtx); err != nil {
		slog.Error("BGP manager stop error", "error", err)
	} else {
		slog.Info("BGP manager stopped")
	}

	// Step 2: Close gRPC connection to central
	slog.Info("closing gRPC connection to central")
	if err := grpcClient.Close(); err != nil {
		slog.Error("gRPC client close error", "error", err)
	} else {
		slog.Info("gRPC connection closed")
	}

	slog.Info("collector stopped gracefully")
}

// waitForBGPSessions polls the BGP manager until at least one peer reaches
// ESTABLISHED state or the timeout expires. This ensures the initial snapshot
// sent to central contains actual routes rather than being empty.
func waitForBGPSessions(ctx context.Context, mgr *bgp.Manager, expected int) {
	if expected == 0 {
		return
	}

	slog.Info("waiting for BGP sessions to establish", "expected", expected)

	timeout := time.After(sessionWaitTimeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			slog.Warn("timed out waiting for BGP sessions, proceeding with available sessions")
			return
		case <-ticker.C:
			count := mgr.EstablishedCount(ctx)
			if count >= expected {
				slog.Info("all BGP sessions established", "count", count)
				return
			}
			if count > 0 {
				slog.Info("BGP sessions partially established",
					"established", count,
					"expected", expected,
				)
			}
		}
	}
}
