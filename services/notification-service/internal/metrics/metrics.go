// Package metrics defines Prometheus metrics for the notification-service.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
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

	// NotificationsSentTotal counts notifications dispatched by channel and status.
	NotificationsSentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tl_notifications_sent_total",
			Help: "Total notifications dispatched, labeled by channel (push, email, in_app) and status.",
		},
		[]string{"channel", "status"},
	)
)
