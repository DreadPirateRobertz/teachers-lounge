// Package metrics provides HTTP middleware for Prometheus instrumentation.
package metrics

import (
	"net/http"
	"strconv"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// HTTPMiddleware records RED metrics for every request.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		status := strconv.Itoa(wrapped.status)
		duration := time.Since(start).Seconds()
		path := r.URL.Path

		HTTPRequestDuration.WithLabelValues(r.Method, path, status).Observe(duration)
		HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
	})
}
