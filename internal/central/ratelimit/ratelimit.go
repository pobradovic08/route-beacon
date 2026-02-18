package ratelimit

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter implements per-IP rate limiting with automatic cleanup of stale entries.
type Limiter struct {
	mu              sync.Mutex
	clients         map[string]*clientEntry
	rate            rate.Limit
	burst           int
	cleanupInterval time.Duration
	staleAfter      time.Duration
	done            chan struct{}
	trustedProxies  map[string]bool
}

type clientEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// New creates a new per-IP rate limiter.
// requestsPerInterval is the number of allowed requests per interval.
// interval is the time window for the rate limit.
// cleanupInterval controls how often stale entries are removed.
// staleAfter is how long a client must be inactive before its entry is removed.
func New(requestsPerInterval int, interval, cleanupInterval, staleAfter time.Duration) (*Limiter, error) {
	if requestsPerInterval <= 0 {
		return nil, fmt.Errorf("ratelimit: requests_per_interval must be positive")
	}
	if interval <= 0 {
		return nil, fmt.Errorf("ratelimit: interval must be positive")
	}
	if cleanupInterval <= 0 {
		return nil, fmt.Errorf("ratelimit: cleanup_interval must be positive")
	}
	if staleAfter <= 0 {
		return nil, fmt.Errorf("ratelimit: stale_after must be positive")
	}

	l := &Limiter{
		clients:         make(map[string]*clientEntry),
		rate:            rate.Limit(float64(requestsPerInterval) / interval.Seconds()),
		burst:           requestsPerInterval,
		cleanupInterval: cleanupInterval,
		staleAfter:      staleAfter,
		done:            make(chan struct{}),
		trustedProxies:  make(map[string]bool),
	}

	go l.cleanupLoop()
	return l, nil
}

// SetTrustedProxies sets the list of trusted proxy IPs for X-Forwarded-For parsing.
func (l *Limiter) SetTrustedProxies(proxies []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.trustedProxies = make(map[string]bool, len(proxies))
	for _, p := range proxies {
		l.trustedProxies[p] = true
	}
}

func (l *Limiter) getClient(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, exists := l.clients[ip]
	if !exists {
		limiter := rate.NewLimiter(l.rate, l.burst)
		l.clients[ip] = &clientEntry{
			limiter:  limiter,
			lastSeen: time.Now(),
		}
		return limiter
	}
	entry.lastSeen = time.Now()
	return entry.limiter
}

// Allow checks if a request from the given IP is allowed.
func (l *Limiter) Allow(ip string) bool {
	return l.getClient(ip).Allow()
}

// RetryAfter returns the number of seconds until the next request from
// this IP would be allowed.
func (l *Limiter) RetryAfter(ip string) int {
	limiter := l.getClient(ip)
	reservation := limiter.Reserve()
	delay := reservation.Delay()
	reservation.Cancel()
	return int(math.Ceil(delay.Seconds()))
}

func (l *Limiter) cleanupLoop() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanup()
		case <-l.done:
			return
		}
	}
}

func (l *Limiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for ip, entry := range l.clients {
		if now.Sub(entry.lastSeen) > l.staleAfter {
			delete(l.clients, ip)
		}
	}
}

// Close stops the cleanup goroutine.
func (l *Limiter) Close() {
	close(l.done)
}

// ExtractClientIP extracts the client IP address from an HTTP request.
// It checks X-Forwarded-For (from trusted proxies), X-Real-IP, and
// falls back to RemoteAddr.
func (l *Limiter) ExtractClientIP(r *http.Request) string {
	// Check X-Forwarded-For from trusted proxies
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		l.mu.Lock()
		trusted := l.trustedProxies[remoteIP]
		l.mu.Unlock()

		if trusted {
			ips := strings.Split(xff, ",")
			// Use the leftmost (client) IP
			clientIP := strings.TrimSpace(ips[0])
			if net.ParseIP(clientIP) != nil {
				return clientIP
			}
		}
	}

	// Check X-Real-IP (only from trusted proxies, same as X-Forwarded-For)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		l.mu.Lock()
		trusted := l.trustedProxies[remoteIP]
		l.mu.Unlock()
		if trusted {
			if net.ParseIP(xri) != nil {
				return xri
			}
		}
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// ProblemDetail represents an RFC 7807 problem response.
type ProblemDetail struct {
	Type       string `json:"type"`
	Title      string `json:"title"`
	Status     int    `json:"status"`
	Detail     string `json:"detail"`
	Instance   string `json:"instance,omitempty"`
	RetryAfter int    `json:"retry_after,omitempty"`
}

// Middleware returns an HTTP middleware that enforces rate limiting.
// When the limit is exceeded, it returns a 429 response with RFC 7807
// Problem Details format including a Retry-After header and remaining
// wait time in the body (per FR-018).
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := l.ExtractClientIP(r)
		if !l.Allow(clientIP) {
			retryAfter := l.RetryAfter(clientIP)
			w.Header().Set("Content-Type", "application/problem+json")
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			problem := ProblemDetail{
				Type:       "about:blank",
				Title:      "Too Many Requests",
				Status:     http.StatusTooManyRequests,
				Detail:     fmt.Sprintf("Rate limit exceeded. Try again in %d seconds.", retryAfter),
				RetryAfter: retryAfter,
			}
			json.NewEncoder(w).Encode(problem)
			return
		}
		next.ServeHTTP(w, r)
	})
}
