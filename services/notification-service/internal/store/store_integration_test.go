package store_test

// Postgres integration tests for Store.
//
// These tests require a live Postgres instance. They are skipped unless
// TEST_DB_URL is set in the environment:
//
//	TEST_DB_URL="postgres://user:pass@localhost:5432/testdb" go test ./internal/store/...
//
// The store functions (CreateNotification, ListUnread, SavePushToken,
// GetPushTokens, Migrate) all operate on a *pgxpool.Pool concrete type and
// cannot be exercised without a real database connection.

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/model"
	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/store"
)

// testDBURL returns the Postgres URL for integration tests or skips.
func testDBURL(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("TEST_DB_URL")
	if addr == "" {
		t.Skip("TEST_DB_URL not set — skipping Postgres integration test")
	}
	return addr
}

// openTestDB opens a pgxpool connection to the test database and runs Migrate.
func openTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := testDBURL(t)
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := store.Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

// TestMigrate_Integration verifies Migrate is idempotent (can run twice).
func TestMigrate_Integration(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// Second call must not fail (IF NOT EXISTS guards).
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatalf("second Migrate call failed: %v", err)
	}
}

// TestCreateAndListNotification_Integration verifies the full round-trip:
// create a notification and retrieve it via ListUnread.
func TestCreateAndListNotification_Integration(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db)
	ctx := context.Background()
	const userID = "store-test-user-1"

	// Clean state.
	db.Exec(ctx, "DELETE FROM notifications WHERE user_id = $1", userID) //nolint:errcheck

	in := &model.Notification{UserID: userID, Type: "xp", Message: "Test notification"}
	created, err := s.CreateNotification(ctx, in)
	if err != nil {
		t.Fatalf("CreateNotification: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty ID after create")
	}
	if created.UserID != userID {
		t.Errorf("UserID = %q, want %q", created.UserID, userID)
	}

	unread, err := s.ListUnread(ctx, userID)
	if err != nil {
		t.Fatalf("ListUnread: %v", err)
	}
	if len(unread) != 1 {
		t.Fatalf("expected 1 unread notification, got %d", len(unread))
	}
	if unread[0].ID != created.ID {
		t.Errorf("ID mismatch: got %q, want %q", unread[0].ID, created.ID)
	}

	db.Exec(ctx, "DELETE FROM notifications WHERE user_id = $1", userID) //nolint:errcheck
}

// TestSaveAndGetPushToken_Integration verifies upsert and retrieval of push tokens.
func TestSaveAndGetPushToken_Integration(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db)
	ctx := context.Background()
	const userID = "store-test-user-2"

	db.Exec(ctx, "DELETE FROM push_tokens WHERE user_id = $1", userID) //nolint:errcheck

	if err := s.SavePushToken(ctx, userID, "tok-abc", "android"); err != nil {
		t.Fatalf("SavePushToken: %v", err)
	}

	// Upsert same token with different platform — should not error.
	if err := s.SavePushToken(ctx, userID, "tok-abc", "ios"); err != nil {
		t.Fatalf("SavePushToken (upsert): %v", err)
	}

	tokens, err := s.GetPushTokens(ctx, userID)
	if err != nil {
		t.Fatalf("GetPushTokens: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token after upsert, got %d", len(tokens))
	}
	if tokens[0] != "tok-abc" {
		t.Errorf("token = %q, want %q", tokens[0], "tok-abc")
	}

	db.Exec(ctx, "DELETE FROM push_tokens WHERE user_id = $1", userID) //nolint:errcheck
}

// TestGetPushTokens_EmptySlice_Integration verifies that GetPushTokens returns
// a non-nil empty slice when no tokens exist for a user.
func TestGetPushTokens_EmptySlice_Integration(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db)
	ctx := context.Background()
	const userID = "store-test-user-no-tokens"

	db.Exec(ctx, "DELETE FROM push_tokens WHERE user_id = $1", userID) //nolint:errcheck

	tokens, err := s.GetPushTokens(ctx, userID)
	if err != nil {
		t.Fatalf("GetPushTokens: %v", err)
	}
	if tokens == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(tokens))
	}
}
