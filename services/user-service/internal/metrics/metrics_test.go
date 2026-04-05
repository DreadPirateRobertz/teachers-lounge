package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teacherslounge/user-service/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestHTTPMiddleware_RecordsStatus(t *testing.T) {
	handler := metrics.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}
}

func TestAuthRequestsTotal_Increments(t *testing.T) {
	before := testutil.ToFloat64(metrics.AuthRequestsTotal.WithLabelValues("login", "success"))
	metrics.AuthRequestsTotal.WithLabelValues("login", "success").Inc()
	after := testutil.ToFloat64(metrics.AuthRequestsTotal.WithLabelValues("login", "success"))

	if after-before != 1.0 {
		t.Fatalf("expected counter increment of 1.0, got %f", after-before)
	}
}

func TestSubscriptionRevenueTotal_Increments(t *testing.T) {
	before := testutil.ToFloat64(metrics.SubscriptionRevenueTotal.WithLabelValues("monthly"))
	metrics.SubscriptionRevenueTotal.WithLabelValues("monthly").Add(999)
	after := testutil.ToFloat64(metrics.SubscriptionRevenueTotal.WithLabelValues("monthly"))

	if after-before != 999.0 {
		t.Fatalf("expected revenue increment of 999.0, got %f", after-before)
	}
}
