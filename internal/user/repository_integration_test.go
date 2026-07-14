package user

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nihongo/internal/db"
)

func testPool(t *testing.T) *pgxpool.Pool {
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
		t.Fatalf("migrations: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func truncate(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `TRUNCATE refresh_tokens, users RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func TestRepository_CreateAndFind(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, "a@b.com", "hash-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 {
		t.Error("expected a non-zero generated id")
	}
	if created.Email != "a@b.com" {
		t.Errorf("email = %q, want a@b.com", created.Email)
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected created_at to be set by the DB")
	}

	byEmail, err := repo.FindByEmail(ctx, "a@b.com")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if byEmail.ID != created.ID {
		t.Errorf("FindByEmail id = %d, want %d", byEmail.ID, created.ID)
	}
	if byEmail.PasswordHash != "hash-1" {
		t.Errorf("password hash = %q, want hash-1", byEmail.PasswordHash)
	}

	byID, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if byID.Email != "a@b.com" {
		t.Errorf("FindByID email = %q, want a@b.com", byID.Email)
	}
}

func TestRepository_DuplicateEmail(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	if _, err := repo.Create(ctx, "dup@b.com", "h1"); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := repo.Create(ctx, "dup@b.com", "h2")
	if !errors.Is(err, ErrEmailTaken) {
		t.Errorf("err = %v, want ErrEmailTaken", err)
	}
}

func TestRepository_FindMissing(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	repo := NewRepository(pool)
	ctx := context.Background()

	if _, err := repo.FindByEmail(ctx, "ghost@b.com"); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByEmail err = %v, want ErrNotFound", err)
	}
	if _, err := repo.FindByID(ctx, 999999); !errors.Is(err, ErrNotFound) {
		t.Errorf("FindByID err = %v, want ErrNotFound", err)
	}
}

func TestRefreshRepository(t *testing.T) {
	pool := testPool(t)
	truncate(t, pool)
	users := NewRepository(pool)
	refresh := NewRefreshRepository(pool)
	ctx := context.Background()

	u, err := users.Create(ctx, "r@b.com", "h")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	expiry := time.Now().Add(time.Hour)

	if err := refresh.Create(ctx, u.ID, "hash-abc", expiry); err != nil {
		t.Fatalf("refresh Create: %v", err)
	}

	got, err := refresh.FindByHash(ctx, "hash-abc")
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if got.UserID != u.ID {
		t.Errorf("user id = %d, want %d", got.UserID, u.ID)
	}
	if got.ExpiresAt.Sub(expiry).Abs() > time.Second {
		t.Errorf("expires_at = %v, want ~%v", got.ExpiresAt, expiry)
	}

	if err := refresh.Delete(ctx, "hash-abc"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := refresh.FindByHash(ctx, "hash-abc"); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete err = %v, want ErrNotFound", err)
	}

	if err := refresh.Delete(ctx, "never-existed"); err != nil {
		t.Errorf("Delete of missing hash should be a no-op, got %v", err)
	}

	if err := refresh.Create(ctx, u.ID, "hash-cascade", expiry); err != nil {
		t.Fatalf("second refresh Create: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, u.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if _, err := refresh.FindByHash(ctx, "hash-cascade"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ON DELETE CASCADE failed: token survived user deletion (err = %v)", err)
	}
}
