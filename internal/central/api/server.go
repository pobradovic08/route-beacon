package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/pobradovic08/route-beacon/internal/central/ratelimit"
)

// Server is the central REST API server.
type Server struct {
	httpServer  *http.Server
	rateLimiter *ratelimit.Limiter
	startedAt   time.Time
}

// ServerDeps holds the dependencies injected into the API server.
type ServerDeps struct {
	Handler     ServerInterface
	RateLimiter *ratelimit.Limiter
	ListenAddr  string
	WriteTimeout time.Duration
	ReadTimeout  time.Duration
}

// NewServer creates a new API server with middleware stack.
func NewServer(deps ServerDeps) *Server {
	s := &Server{
		rateLimiter: deps.RateLimiter,
		startedAt:   time.Now(),
	}

	// Create handler from the generated interface
	apiHandler := Handler(deps.Handler)

	// Build middleware stack
	handler := s.withPanicRecovery(apiHandler)
	handler = s.withRateLimit(handler)
	handler = s.withLogging(handler)
	handler = s.withCORS(handler)

	s.httpServer = &http.Server{
		Addr:         deps.ListenAddr,
		Handler:      handler,
		WriteTimeout: deps.WriteTimeout,
		ReadTimeout:  deps.ReadTimeout,
	}

	return s
}

// Start begins listening for HTTP requests.
func (s *Server) Start() error {
	slog.Info("starting API server", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("API server: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the API server.
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("shutting down API server")
	return s.httpServer.Shutdown(ctx)
}

// UptimeSeconds returns the number of seconds since the server started.
func (s *Server) UptimeSeconds() int {
	return int(time.Since(s.startedAt).Seconds())
}

// Middleware: structured logging
func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration", time.Since(start),
			"remote_addr", r.RemoteAddr,
		)
	})
}

// Middleware: CORS headers
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Middleware: rate limiting for diagnostic endpoints and route lookups.
// Per OpenAPI spec, ping, traceroute (POST), and route lookup (GET) share
// a combined per-IP rate limit. Other GET endpoints are not rate-limited.
func (s *Server) withRateLimit(next http.Handler) http.Handler {
	if s.rateLimiter == nil {
		return next
	}
	limited := s.rateLimiter.Middleware(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || strings.Contains(r.URL.Path, "/routes/lookup") {
			limited.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Middleware: panic recovery
func (s *Server) withPanicRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered in HTTP handler",
					"error", err,
					"path", r.URL.Path,
				)
				WriteProblem(w, http.StatusInternalServerError, "Internal Server Error",
					"An unexpected error occurred. Please try again later.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// WriteProblem writes an RFC 7807 Problem Details JSON response.
func WriteProblem(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ProblemDetail{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Detail: &detail,
	})
}

// WriteJSON writes a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// SSEWriter provides helpers for writing Server-Sent Events.
type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewSSEWriter initializes an SSE stream on the response.
func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	return &SSEWriter{w: w, flusher: flusher}, nil
}

// WriteEvent sends a single SSE event.
func (s *SSEWriter) WriteEvent(event string, data any) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal SSE data: %w", err)
	}
	fmt.Fprintf(s.w, "event: %s\ndata: %s\n\n", event, jsonData)
	s.flusher.Flush()
	return nil
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
