// Package metrics defines Prometheus metrics for the gaming-service.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// XPGrantedTotal counts total XP awarded to students.
	XPGrantedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tl_xp_granted_total",
			Help: "Total XP granted to students across all sessions.",
		},
	)

	// BossBattlesTotal counts boss battle outcomes by result label.
	BossBattlesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tl_boss_battles_total",
			Help: "Total boss battles completed, labeled by result (win, loss, forfeit).",
		},
		[]string{"result"},
	)

	// ActiveStreaksGauge tracks the current number of active daily streaks.
	ActiveStreaksGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "tl_active_streaks_gauge",
			Help: "Number of students with an active daily streak.",
		},
	)

	// HTTPRequestDuration tracks latency of all HTTP requests.
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestsTotal counts all HTTP requests.
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)
)
