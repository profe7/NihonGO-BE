package hiragana

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"nihongo/internal/db"
	"nihongo/internal/user"
)

func TestRepository_Progress(t *testing.T) {
	pool := hiraganaTestPool(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx,
		`TRUNCATE hiragana_attempts, refresh_tokens, users RESTART IDENTITY CASCADE`,
	)
	if err != nil {
		t.Fatalf("truncate tables: %v", err)
	}

	users := user.NewRepository(pool)
	createdUser, err := users.Create(ctx, "progress@example.com", "password-hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	repository := NewRepository(pool)

	progress, err := repository.Progress(ctx, createdUser.ID)
	if err != nil {
		t.Fatalf("load empty progress: %v", err)
	}
	if progress.TotalAttempts != 0 || progress.CorrectAttempts != 0 {
		t.Fatalf("empty progress = %+v; want zero values", progress)
	}

	card, err := repository.Random(ctx, nil)
	if err != nil {
		t.Fatalf("load hiragana card: %v", err)
	}

	for _, correct := range []bool{true, true, false} {
		if err := repository.RecordAttempt(
			ctx,
			createdUser.ID,
			card.ID,
			correct,
		); err != nil {
			t.Fatalf("record attempt: %v", err)
		}
	}

	progress, err = repository.Progress(ctx, createdUser.ID)
	if err != nil {
		t.Fatalf("load progress: %v", err)
	}
	if progress.TotalAttempts != 3 {
		t.Errorf("TotalAttempts = %d; want 3", progress.TotalAttempts)
	}
	if progress.CorrectAttempts != 2 {
		t.Errorf("CorrectAttempts = %d; want 2", progress.CorrectAttempts)
	}
}

func hiraganaTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	if err := db.RunMigrations(ctx, pool, os.DirFS("../..")); err != nil {
		pool.Close()
		t.Fatalf("migrations: %v", err)
	}

	t.Cleanup(pool.Close)
	return pool
}
