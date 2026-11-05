package db_test

import (
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/peerbenmoshe/madam/backend/internal/db"
)

// testPool connects to the database in MADAM_TEST_DATABASE_URL, or skips the
// test when it is unset. Set but unreachable is a failure, not a skip, so CI
// can't pass by quietly losing its database. `make test-integration` and CI
// set it to a migrated Postgres, connecting as the application role
// madam_app, the role the server uses.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("MADAM_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("MADAM_TEST_DATABASE_URL is not set; run make test-integration")
	}
	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestPing(t *testing.T) {
	got, err := db.New(testPool(t)).Ping(t.Context())
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if got != 1 {
		t.Errorf("Ping = %d, want 1", got)
	}
}
