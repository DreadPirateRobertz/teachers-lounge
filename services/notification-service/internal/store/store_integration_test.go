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
	"time"

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

// ── Streak-risk SQL integration tests (tl-wti) ────────────────────────────────
//
// These tests exercise GetUsersAtRiskOfStreakLoss against a real Postgres
// instance. Each test inserts a gaming_profiles row with specific field values
// and asserts whether the row appears in (or is excluded from) the result set.
//
// Prerequisites: the gaming_profiles table must exist. Run Migrate first (done
// by openTestDB) which adds last_streak_reminder_at if it does not exist yet.
// The gaming_profiles table itself is created by gaming-service; the test
// creates a minimal version when it does not exist.

// ensureGamingProfilesTable creates a minimal gaming_profiles table when it
// does not already exist, so integration tests can run against a fresh DB
// without requiring the full gaming-service migration to have run.
func ensureGamingProfilesTable(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	const ddl = `
		CREATE TABLE IF NOT EXISTS gaming_profiles (
			user_id               TEXT        PRIMARY KEY,
			current_streak        INT         NOT NULL DEFAULT 0,
			longest_streak        INT         NOT NULL DEFAULT 0,
			last_study_date       TIMESTAMPTZ,
			streak_frozen_until   TIMESTAMPTZ,
			last_streak_reminder_at TIMESTAMPTZ,
			xp                    INT         NOT NULL DEFAULT 0,
			level                 INT         NOT NULL DEFAULT 1,
			gems                  INT         NOT NULL DEFAULT 0,
			bosses_defeated       INT         NOT NULL DEFAULT 0,
			power_ups             JSONB       NOT NULL DEFAULT '{}'::jsonb,
			cosmetics             JSONB       NOT NULL DEFAULT '{}'::jsonb
		)`
	if _, err := db.Exec(context.Background(), ddl); err != nil {
		t.Fatalf("ensureGamingProfilesTable: %v", err)
	}
}

// upsertGamingProfile inserts or replaces a minimal gaming_profiles row for
// the given user with the provided field values. Fields not specified are set
// to their DEFAULT.
func upsertGamingProfile(t *testing.T, db *pgxpool.Pool, userID string, currentStreak int,
	lastStudyDate *time.Time, streakFrozenUntil *time.Time, lastStreakReminderAt *time.Time,
) {
	t.Helper()
	const q = `
		INSERT INTO gaming_profiles
			(user_id, current_streak, last_study_date, streak_frozen_until, last_streak_reminder_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET current_streak          = EXCLUDED.current_streak,
		    last_study_date         = EXCLUDED.last_study_date,
		    streak_frozen_until     = EXCLUDED.streak_frozen_until,
		    last_streak_reminder_at = EXCLUDED.last_streak_reminder_at`
	if _, err := db.Exec(context.Background(), q,
		userID, currentStreak, lastStudyDate, streakFrozenUntil, lastStreakReminderAt,
	); err != nil {
		t.Fatalf("upsertGamingProfile(%s): %v", userID, err)
	}
}

// containsUser reports whether users contains a row with the given userID.
func containsUser(users []model.UserAtRisk, userID string) bool {
	for _, u := range users {
		if u.UserID == userID {
			return true
		}
	}
	return false
}

// TestStreakRisk_UserAt20h_Appears verifies that a user whose last_study_date
// is exactly 20h ago (the lower boundary of the reminder window) is included.
func TestStreakRisk_UserAt20h_Appears(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db)
	ctx := context.Background()
	const userID = "streak-risk-at-20h"

	ensureGamingProfilesTable(t, db)
	t.Cleanup(func() {
		db.Exec(ctx, "DELETE FROM gaming_profiles WHERE user_id = $1", userID) //nolint:errcheck
	})

	// Exactly 20h ago — lower boundary (inclusive).
	lastStudy := time.Now().UTC().Add(-20 * time.Hour)
	upsertGamingProfile(t, db, userID, 5, &lastStudy, nil, nil)

	users, err := s.GetUsersAtRiskOfStreakLoss(ctx, 20, 24)
	if err != nil {
		t.Fatalf("GetUsersAtRiskOfStreakLoss: %v", err)
	}
	if !containsUser(users, userID) {
		t.Errorf("user at exactly 20h should appear in at-risk set, got %v", users)
	}
}

// TestStreakRisk_UserAt24h_Excluded verifies that a user whose last_study_date
// is exactly 24h ago (the upper boundary, exclusive) is NOT included — they
// have already lost their streak.
func TestStreakRisk_UserAt24h_Excluded(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db)
	ctx := context.Background()
	const userID = "streak-risk-at-24h"

	ensureGamingProfilesTable(t, db)
	t.Cleanup(func() {
		db.Exec(ctx, "DELETE FROM gaming_profiles WHERE user_id = $1", userID) //nolint:errcheck
	})

	// Exactly 24h ago — upper boundary (exclusive: NOT > NOW()-24h).
	lastStudy := time.Now().UTC().Add(-24 * time.Hour)
	upsertGamingProfile(t, db, userID, 3, &lastStudy, nil, nil)

	users, err := s.GetUsersAtRiskOfStreakLoss(ctx, 20, 24)
	if err != nil {
		t.Fatalf("GetUsersAtRiskOfStreakLoss: %v", err)
	}
	if containsUser(users, userID) {
		t.Errorf("user at exactly 24h should NOT appear in at-risk set (already lapsed)")
	}
}

// TestStreakRisk_ZeroStreak_Excluded verifies that a user with current_streak=0
// is excluded — there is no streak to protect.
func TestStreakRisk_ZeroStreak_Excluded(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db)
	ctx := context.Background()
	const userID = "streak-risk-zero-streak"

	ensureGamingProfilesTable(t, db)
	t.Cleanup(func() {
		db.Exec(ctx, "DELETE FROM gaming_profiles WHERE user_id = $1", userID) //nolint:errcheck
	})

	lastStudy := time.Now().UTC().Add(-22 * time.Hour)
	upsertGamingProfile(t, db, userID, 0, &lastStudy, nil, nil)

	users, err := s.GetUsersAtRiskOfStreakLoss(ctx, 20, 24)
	if err != nil {
		t.Fatalf("GetUsersAtRiskOfStreakLoss: %v", err)
	}
	if containsUser(users, userID) {
		t.Errorf("user with current_streak=0 should NOT appear in at-risk set")
	}
}

// TestStreakRisk_FrozenStreak_Excluded verifies that a user with an active
// streak freeze (streak_frozen_until in the future) is excluded.
func TestStreakRisk_FrozenStreak_Excluded(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db)
	ctx := context.Background()
	const userID = "streak-risk-frozen"

	ensureGamingProfilesTable(t, db)
	t.Cleanup(func() {
		db.Exec(ctx, "DELETE FROM gaming_profiles WHERE user_id = $1", userID) //nolint:errcheck
	})

	lastStudy := time.Now().UTC().Add(-22 * time.Hour)
	frozenUntil := time.Now().UTC().Add(2 * time.Hour) // still active
	upsertGamingProfile(t, db, userID, 7, &lastStudy, &frozenUntil, nil)

	users, err := s.GetUsersAtRiskOfStreakLoss(ctx, 20, 24)
	if err != nil {
		t.Fatalf("GetUsersAtRiskOfStreakLoss: %v", err)
	}
	if containsUser(users, userID) {
		t.Errorf("user with active streak freeze should NOT appear in at-risk set")
	}
}

// TestStreakRisk_NullLastStudyDate_Excluded verifies that a user with a NULL
// last_study_date is excluded — they have never studied, so there is no streak
// to lose.
func TestStreakRisk_NullLastStudyDate_Excluded(t *testing.T) {
	db := openTestDB(t)
	s := store.New(db)
	ctx := context.Background()
	const userID = "streak-risk-null-last-study"

	ensureGamingProfilesTable(t, db)
	t.Cleanup(func() {
		db.Exec(ctx, "DELETE FROM gaming_profiles WHERE user_id = $1", userID) //nolint:errcheck
	})

	upsertGamingProfile(t, db, userID, 4, nil, nil, nil)

	users, err := s.GetUsersAtRiskOfStreakLoss(ctx, 20, 24)
	if err != nil {
		t.Fatalf("GetUsersAtRiskOfStreakLoss: %v", err)
	}
	if containsUser(users, userID) {
		t.Errorf("user with NULL last_study_date should NOT appear in at-risk set")
	}
}
