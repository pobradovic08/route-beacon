package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pobradovic08/route-beacon/internal/handler"
	"github.com/pobradovic08/route-beacon/internal/store"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := store.NewDB(ctx, databaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	startTime := time.Now()

	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /api/v1/health", handler.HandleGetHealth(db, startTime))

	// Routers
	mux.HandleFunc("GET /api/v1/routers", handler.HandleListRouters(db))
	mux.HandleFunc("GET /api/v1/routers/{routerId}", handler.HandleGetRouter(db))

	// Route lookup
	mux.HandleFunc("GET /api/v1/routers/{routerId}/routes/lookup", handler.HandleLookupRoutes(db))

	// Route history
	mux.HandleFunc("GET /api/v1/routers/{routerId}/routes/history", handler.HandleGetRouteHistory(db))

	// Apply middleware stack: Logger → Recover → CORS → JSON → mux
	h := handler.Logger(handler.Recover(handler.CORS(handler.JSON(mux))))

	srv := &http.Server{
		Addr:    listenAddr,
		Handler: h,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("listening on %s", listenAddr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server: %v", err)
	}
}
