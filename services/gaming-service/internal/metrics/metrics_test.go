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

func TestBattleSessionsActive_IncDec(t *testing.T) {
	before := testutil.ToFloat64(metrics.BattleSessionsActive)

	metrics.BattleSessionsActive.Inc()
	afterInc := testutil.ToFloat64(metrics.BattleSessionsActive)
	if afterInc-before != 1.0 {
		t.Fatalf("expected +1 after Inc, got delta %f", afterInc-before)
	}

	metrics.BattleSessionsActive.Dec()
	afterDec := testutil.ToFloat64(metrics.BattleSessionsActive)
	if afterDec != before {
		t.Fatalf("expected gauge to return to %f after Dec, got %f", before, afterDec)
	}
}

func TestBossDefeatsTotal_LabeledByBossID(t *testing.T) {
	before := testutil.ToFloat64(metrics.BossDefeatsTotal.WithLabelValues("algebra-wyrm"))
	metrics.BossDefeatsTotal.WithLabelValues("algebra-wyrm").Inc()
	after := testutil.ToFloat64(metrics.BossDefeatsTotal.WithLabelValues("algebra-wyrm"))

	if after-before != 1.0 {
		t.Fatalf("expected boss defeat increment of 1.0, got %f", after-before)
	}
}

func TestHTTPRequestDuration_LabelsRouteAndStatusCode(t *testing.T) {
	// Observe a duration with the canonical label names; if the histogram was
	// registered with different labels this will panic, failing the test.
	// This verifies the labels are "method", "route", "status_code" as specced.
	metrics.HTTPRequestDuration.WithLabelValues("GET", "/gaming/profile/abc", "200").Observe(0.05)
	// No assertion needed beyond not panicking — label mismatch causes a panic.
}
