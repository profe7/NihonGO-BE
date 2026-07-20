package testutil

import (
	"context"
	"io/fs"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"nihongo/internal/db"
)

func OpenDatabase(
	t *testing.T,
	migrationsFS fs.FS,
) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}

	if err := db.RunMigrations(ctx, pool, migrationsFS); err != nil {
		pool.Close()
		t.Fatalf("run test migrations: %v", err)
	}

	t.Cleanup(pool.Close)
	return pool
}
