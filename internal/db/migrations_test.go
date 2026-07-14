package db

import (
	"context"
	"os"
	"testing"
)

func TestRunMigrations_Idempotent(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()

	pool, err := Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	fsys := os.DirFS("../..")

	if err := RunMigrations(ctx, pool, fsys); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := RunMigrations(ctx, pool, fsys); err != nil {
		t.Fatalf("second run should be a no-op, got: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count < 2 {
		t.Errorf("recorded migrations = %d, want >= 2", count)
	}

	var tables int
	err = pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name IN ('users', 'refresh_tokens')
	`).Scan(&tables)
	if err != nil {
		t.Fatalf("check tables: %v", err)
	}
	if tables != 2 {
		t.Errorf("expected users + refresh_tokens tables, found %d", tables)
	}
}
