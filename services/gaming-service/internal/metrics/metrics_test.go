package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/teacherslounge/gaming-service/internal/metrics"
)

func TestHTTPMiddleware_RecordsStatus(t *testing.T) {
	handler := metrics.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/gaming/profile/abc", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestXPGrantedTotal_Increments(t *testing.T) {
	before := testutil.ToFloat64(metrics.XPGrantedTotal)
	metrics.XPGrantedTotal.Add(100)
	after := testutil.ToFloat64(metrics.XPGrantedTotal)

	if after-before != 100.0 {
		t.Fatalf("expected XP increment of 100.0, got %f", after-before)
	}
}

func TestBossBattlesTotal_LabeledByResult(t *testing.T) {
	before := testutil.ToFloat64(metrics.BossBattlesTotal.WithLabelValues("win"))
	metrics.BossBattlesTotal.WithLabelValues("win").Inc()
	after := testutil.ToFloat64(metrics.BossBattlesTotal.WithLabelValues("win"))

	if after-before != 1.0 {
		t.Fatalf("expected battle win increment of 1.0, got %f", after-before)
	}
}

func TestActiveStreaksGauge_SetAndRead(t *testing.T) {
	metrics.ActiveStreaksGauge.Set(42)
	val := testutil.ToFloat64(metrics.ActiveStreaksGauge)
	if val != 42.0 {
		t.Fatalf("expected active streaks gauge of 42.0, got %f", val)
	}
}
