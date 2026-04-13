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
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		statusCode := strconv.Itoa(wrapped.status)
		duration := time.Since(start).Seconds()
		route := r.URL.Path

		HTTPRequestDuration.WithLabelValues(r.Method, route, statusCode).Observe(duration)
		HTTPRequestsTotal.WithLabelValues(r.Method, route, statusCode).Inc()
	})
}
