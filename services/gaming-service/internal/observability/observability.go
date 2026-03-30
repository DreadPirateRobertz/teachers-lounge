// Package observability provides OpenTelemetry tracing and Prometheus metrics
// for the gaming-service.
package observability

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"http_request_method", "http_route", "http_response_status_code"},
	)

	activeRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "http_server_active_requests",
			Help: "Number of active HTTP requests.",
		},
		[]string{"http_request_method"},
	)

	dbQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
		},
		[]string{"operation"},
	)
)

func init() {
	prometheus.MustRegister(requestDuration, activeRequests, dbQueryDuration)
}

// RecordDBQuery records a database query duration metric.
func RecordDBQuery(operation string, duration time.Duration) {
	dbQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// InitTracer sets up the OpenTelemetry trace provider with an OTLP gRPC exporter.
func InitTracer(ctx context.Context, serviceName, otelEndpoint string) (func(context.Context) error, error) {
	if otelEndpoint == "" {
		otelEndpoint = "localhost:4317"
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otelEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP trace exporter: %w", err)
	}

	res, err := sdkresource.New(ctx,
		sdkresource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// MetricsHandler returns an http.Handler that serves Prometheus metrics.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// Middleware returns a chi-compatible middleware that records HTTP metrics and
// creates trace spans for each request.
func Middleware(serviceName string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path,
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(r.Method),
					semconv.URLPath(r.URL.Path),
				),
			)
			defer span.End()

			activeRequests.WithLabelValues(r.Method).Inc()
			defer activeRequests.WithLabelValues(r.Method).Dec()

			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(ww, r.WithContext(ctx))

			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = r.URL.Path
			}

			duration := time.Since(start)
			statusStr := fmt.Sprintf("%d", ww.statusCode)

			requestDuration.WithLabelValues(r.Method, route, statusStr).Observe(duration.Seconds())

			span.SetAttributes(
				attribute.Int("http.response.status_code", ww.statusCode),
				attribute.Float64("http.response.duration_s", duration.Seconds()),
			)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}
