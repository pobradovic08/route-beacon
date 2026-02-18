package api

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsCollector holds all Prometheus metrics for the route-beacon central server.
type MetricsCollector struct {
	RoutesTotal              *prometheus.GaugeVec
	CollectorsConnected      prometheus.Gauge
	SyncSnapshotDuration     prometheus.Histogram
	SyncUpdatesTotal         prometheus.Counter
	CommandsTotal            *prometheus.CounterVec
	RateLimitRejectionsTotal prometheus.Counter
}

// NewMetricsCollector creates and registers all Prometheus metrics.
func NewMetricsCollector() *MetricsCollector {
	m := &MetricsCollector{
		RoutesTotal: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "routebeacon_routes_total",
				Help: "Total number of routes per collector, router, and address family.",
			},
			[]string{"collector", "router", "afi"},
		),
		CollectorsConnected: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "routebeacon_collectors_connected",
				Help: "Number of currently connected collectors.",
			},
		),
		SyncSnapshotDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "routebeacon_sync_snapshot_duration_seconds",
				Help:    "Duration of snapshot sync operations in seconds.",
				Buckets: prometheus.DefBuckets,
			},
		),
		SyncUpdatesTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "routebeacon_sync_updates_total",
				Help: "Total number of route sync updates received.",
			},
		),
		CommandsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "routebeacon_commands_total",
				Help: "Total number of commands dispatched to collectors.",
			},
			[]string{"type", "status"},
		),
		RateLimitRejectionsTotal: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "routebeacon_ratelimit_rejections_total",
				Help: "Total number of requests rejected by rate limiting.",
			},
		),
	}

	prometheus.MustRegister(
		m.RoutesTotal,
		m.CollectorsConnected,
		m.SyncSnapshotDuration,
		m.SyncUpdatesTotal,
		m.CommandsTotal,
		m.RateLimitRejectionsTotal,
	)

	return m
}

// SetRoutesTotal sets the route count gauge for a specific collector/router/afi combination.
func (m *MetricsCollector) SetRoutesTotal(collector, router, afi string, count float64) {
	m.RoutesTotal.WithLabelValues(collector, router, afi).Set(count)
}

// SetCollectorsConnected sets the current number of connected collectors.
func (m *MetricsCollector) SetCollectorsConnected(count float64) {
	m.CollectorsConnected.Set(count)
}

// ObserveSyncSnapshotDuration records the duration of a snapshot sync.
func (m *MetricsCollector) ObserveSyncSnapshotDuration(seconds float64) {
	m.SyncSnapshotDuration.Observe(seconds)
}

// IncSyncUpdatesTotal increments the sync updates counter.
func (m *MetricsCollector) IncSyncUpdatesTotal() {
	m.SyncUpdatesTotal.Inc()
}

// IncCommandsTotal increments the commands counter for a given type and status.
func (m *MetricsCollector) IncCommandsTotal(cmdType, status string) {
	m.CommandsTotal.WithLabelValues(cmdType, status).Inc()
}

// IncRateLimitRejectionsTotal increments the rate limit rejection counter.
func (m *MetricsCollector) IncRateLimitRejectionsTotal() {
	m.RateLimitRejectionsTotal.Inc()
}

// Handler returns an http.Handler that serves Prometheus metrics.
func (m *MetricsCollector) Handler() http.Handler {
	return promhttp.Handler()
}
