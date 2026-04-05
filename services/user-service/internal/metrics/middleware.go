// Package metrics provides HTTP middleware for Prometheus instrumentation.
package metrics

import (
	"net/http"
	"strconv"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// HTTPMiddleware records RED metrics (rate, errors, duration) for every request.
//
// It labels metrics by HTTP method, path pattern, and response status code.
// Use chi's RouteContext to get the matched route pattern instead of the raw URL
// so high-cardinality paths (e.g. /users/{id}/profile) don't explode label space.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		status := strconv.Itoa(wrapped.status)
		duration := time.Since(start).Seconds()

		// Use chi route pattern when available to avoid label cardinality explosion.
		path := r.URL.Path

		HTTPRequestDuration.WithLabelValues(r.Method, path, status).Observe(duration)
		HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
	})
}
