package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestHTTPMiddleware_RecordsStatus(t *testing.T) {
	handler := metrics.HTTPMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/notify/push", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestNotificationsSentTotal_IncrementsByChannel(t *testing.T) {
	before := testutil.ToFloat64(metrics.NotificationsSentTotal.WithLabelValues("push", "success"))
	metrics.NotificationsSentTotal.WithLabelValues("push", "success").Inc()
	after := testutil.ToFloat64(metrics.NotificationsSentTotal.WithLabelValues("push", "success"))

	if after-before != 1.0 {
		t.Fatalf("expected notification increment of 1.0, got %f", after-before)
	}
}

func TestNotificationsSentTotal_SeparateLabels(t *testing.T) {
	pushBefore := testutil.ToFloat64(metrics.NotificationsSentTotal.WithLabelValues("email", "success"))
	metrics.NotificationsSentTotal.WithLabelValues("email", "success").Inc()
	metrics.NotificationsSentTotal.WithLabelValues("email", "failure").Inc()
	pushAfter := testutil.ToFloat64(metrics.NotificationsSentTotal.WithLabelValues("email", "success"))

	if pushAfter-pushBefore != 1.0 {
		t.Fatalf("expected email success increment of 1.0, got %f", pushAfter-pushBefore)
	}
}
