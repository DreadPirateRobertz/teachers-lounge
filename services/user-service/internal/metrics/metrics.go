// Package metrics defines Prometheus metrics for the user-service.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// AuthRequestsTotal counts authentication requests by status label.
	AuthRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tl_auth_requests_total",
			Help: "Total number of auth requests (login, register, refresh) by status.",
		},
		[]string{"operation", "status"},
	)

	// SubscriptionRevenueTotal tracks cumulative subscription revenue in USD cents.
	SubscriptionRevenueTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tl_subscription_revenue_total",
			Help: "Cumulative subscription revenue in USD cents by plan.",
		},
		[]string{"plan"},
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
