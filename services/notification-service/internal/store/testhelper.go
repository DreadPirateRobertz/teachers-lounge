package store

import (
	"os"
	"testing"
)

// TestRedisAddr returns the Redis address from TEST_REDIS_ADDR env var,
// or empty string if not set. Used by integration tests.
func TestRedisAddr(t *testing.T) string {
	t.Helper()
	return os.Getenv("TEST_REDIS_ADDR")
}
