package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/handler"
	"github.com/teacherslounge/gaming-service/internal/middleware"
	"github.com/teacherslounge/gaming-service/internal/model"
	"github.com/teacherslounge/gaming-service/internal/taunt"
)

var _ handler.Storer = (*shopStore)(nil)

// ── shopStore stub ────────────────────────────────────────────────────────────

// shopStore overrides BuyPowerUp on top of battleStore (which already satisfies
// the full Storer interface).
type shopStore struct {
	battleStore
	gemsLeft int
	newCount int
	buyErr   error
}

func (s *shopStore) BuyPowerUp(_ context.Context, _ string, _ model.PowerUpType, _ int) (int, int, error) {
	return s.gemsLeft, s.newCount, s.buyErr
}

func newShopHandler(s handler.Storer) *handler.Handler {
	return handler.New(s, taunt.StaticGenerator{}, zap.NewNop())
}

// withUser injects a caller user ID into the request context.
func withUser(r *http.Request, userID string) *http.Request {
	ctx := middleware.WithUserID(r.Context(), userID)
	return r.WithContext(ctx)
}

// ── GetShopCatalog ────────────────────────────────────────────────────────────

func TestGetShopCatalog_ReturnsFourItems(t *testing.T) {
	h := newShopHandler(&shopStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/shop/catalog", nil)
	rec := httptest.NewRecorder()

	h.GetShopCatalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var catalog model.ShopCatalog
	if err := json.NewDecoder(rec.Body).Decode(&catalog); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(catalog.Items) != 4 {
		t.Errorf("expected 4 items, got %d", len(catalog.Items))
	}
}

func TestGetShopCatalog_AllItemsHavePositiveCost(t *testing.T) {
	h := newShopHandler(&shopStore{})
	req := httptest.NewRequest(http.MethodGet, "/gaming/shop/catalog", nil)
	rec := httptest.NewRecorder()

	h.GetShopCatalog(rec, req)

	var catalog model.ShopCatalog
	_ = json.NewDecoder(rec.Body).Decode(&catalog)
	for _, item := range catalog.Items {
		if item.GemCost <= 0 {
			t.Errorf("item %s has non-positive gem cost %d", item.Type, item.GemCost)
		}
	}
}

// ── BuyPowerUp ────────────────────────────────────────────────────────────────

func buyBody(userID string, pu model.PowerUpType) *bytes.Buffer {
	b, _ := json.Marshal(model.BuyPowerUpRequest{UserID: userID, PowerUp: pu})
	return bytes.NewBuffer(b)
}

func TestBuyPowerUp_Success(t *testing.T) {
	st := &shopStore{gemsLeft: 7, newCount: 2}
	h := newShopHandler(st)

	req := httptest.NewRequest(http.MethodPost, "/gaming/shop/buy", buyBody("user1", model.PowerUpShield))
	req = withUser(req, "user1")
	rec := httptest.NewRecorder()

	h.BuyPowerUp(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp model.BuyPowerUpResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.PowerUp != model.PowerUpShield {
		t.Errorf("expected shield, got %s", resp.PowerUp)
	}
	if resp.GemsLeft != 7 {
		t.Errorf("expected 7 gems left, got %d", resp.GemsLeft)
	}
	if resp.NewCount != 2 {
		t.Errorf("expected new_count=2, got %d", resp.NewCount)
	}
}

func TestBuyPowerUp_InsufficientGems(t *testing.T) {
	st := &shopStore{buyErr: errors.New("not enough gems")}
	h := newShopHandler(st)

	req := httptest.NewRequest(http.MethodPost, "/gaming/shop/buy", buyBody("user1", model.PowerUpCritical))
	req = withUser(req, "user1")
	rec := httptest.NewRecorder()

	h.BuyPowerUp(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rec.Code)
	}
}

func TestBuyPowerUp_UnknownType(t *testing.T) {
	h := newShopHandler(&shopStore{})

	req := httptest.NewRequest(http.MethodPost, "/gaming/shop/buy", buyBody("user1", "laser_beam"))
	req = withUser(req, "user1")
	rec := httptest.NewRecorder()

	h.BuyPowerUp(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestBuyPowerUp_Forbidden_WrongUser(t *testing.T) {
	h := newShopHandler(&shopStore{})

	req := httptest.NewRequest(http.MethodPost, "/gaming/shop/buy", buyBody("user-a", model.PowerUpHeal))
	req = withUser(req, "user-b") // caller ≠ request user
	rec := httptest.NewRecorder()

	h.BuyPowerUp(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestBuyPowerUp_Forbidden_NoAuth(t *testing.T) {
	h := newShopHandler(&shopStore{})

	req := httptest.NewRequest(http.MethodPost, "/gaming/shop/buy", buyBody("user1", model.PowerUpHeal))
	// no withUser call → empty caller ID
	rec := httptest.NewRecorder()

	h.BuyPowerUp(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestBuyPowerUp_InvalidBody(t *testing.T) {
	h := newShopHandler(&shopStore{})

	req := httptest.NewRequest(http.MethodPost, "/gaming/shop/buy", bytes.NewBufferString("not json"))
	req = withUser(req, "user1")
	rec := httptest.NewRecorder()

	h.BuyPowerUp(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestBuyPowerUp_AllValidTypes(t *testing.T) {
	types := []model.PowerUpType{
		model.PowerUpShield,
		model.PowerUpDoubleDamage,
		model.PowerUpHeal,
		model.PowerUpCritical,
	}
	for _, pu := range types {
		t.Run(string(pu), func(t *testing.T) {
			st := &shopStore{gemsLeft: 10, newCount: 1}
			h := newShopHandler(st)
			req := httptest.NewRequest(http.MethodPost, "/gaming/shop/buy", buyBody("user1", pu))
			req = withUser(req, "user1")
			rec := httptest.NewRecorder()

			h.BuyPowerUp(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected 200 for %s, got %d: %s", pu, rec.Code, rec.Body.String())
			}
		})
	}
}
