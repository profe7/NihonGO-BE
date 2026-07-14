package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"nihongo/internal/user"
)

func TestRefreshService_Issue(t *testing.T) {
	var gotUser int64
	var gotHash string
	var gotExpiry time.Time
	store := fakeRefreshStore{
		createFn: func(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
			gotUser = userID
			gotHash = tokenHash
			gotExpiry = expiresAt
			return nil
		},
	}
	svc := NewRefreshService(store, time.Hour)

	raw, err := svc.Issue(context.Background(), 5)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if raw == "" {
		t.Fatal("expected a non-empty raw token")
	}
	if gotUser != 5 {
		t.Errorf("stored user id = %d, want 5", gotUser)
	}
	if gotHash == raw {
		t.Error("stored the raw token; it must store the hash instead")
	}
	if gotHash != hashToken(raw) {
		t.Error("stored hash does not match hashToken(raw)")
	}
	if !gotExpiry.After(time.Now()) {
		t.Error("expiry should be in the future")
	}
}

func TestRefreshService_RotateHappyPath(t *testing.T) {
	var deletedHashes []string
	created := 0
	store := fakeRefreshStore{
		findByHashFn: func(ctx context.Context, hash string) (user.RefreshToken, error) {
			return user.RefreshToken{ID: 1, UserID: 9, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)}, nil
		},
		deleteFn: func(ctx context.Context, hash string) error {
			deletedHashes = append(deletedHashes, hash)
			return nil
		},
		createFn: func(ctx context.Context, userID int64, hash string, exp time.Time) error {
			created++
			return nil
		},
	}
	svc := NewRefreshService(store, time.Hour)

	newRaw, userID, err := svc.Rotate(context.Background(), "old-raw")
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if userID != 9 {
		t.Errorf("user id = %d, want 9", userID)
	}
	if newRaw == "" {
		t.Error("expected a new raw token")
	}
	if len(deletedHashes) != 1 {
		t.Fatalf("delete calls = %d, want 1", len(deletedHashes))
	}
	if deletedHashes[0] != hashToken("old-raw") {
		t.Error("rotation deleted the wrong token hash")
	}
	if created != 1 {
		t.Errorf("create calls = %d, want 1", created)
	}
}

func TestRefreshService_RotateRejectsMissing(t *testing.T) {
	store := fakeRefreshStore{
		findByHashFn: func(ctx context.Context, hash string) (user.RefreshToken, error) {
			return user.RefreshToken{}, user.ErrNotFound
		},
	}
	svc := NewRefreshService(store, time.Hour)

	if _, _, err := svc.Rotate(context.Background(), "whatever"); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("err = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestRefreshService_RotateRejectsExpired(t *testing.T) {
	deleted := 0
	store := fakeRefreshStore{
		findByHashFn: func(ctx context.Context, hash string) (user.RefreshToken, error) {
			return user.RefreshToken{UserID: 3, TokenHash: hash, ExpiresAt: time.Now().Add(-time.Minute)}, nil
		},
		deleteFn: func(ctx context.Context, hash string) error {
			deleted++
			return nil
		},
	}
	svc := NewRefreshService(store, time.Hour)

	if _, _, err := svc.Rotate(context.Background(), "expired"); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("err = %v, want ErrInvalidRefreshToken", err)
	}
	if deleted != 1 {
		t.Errorf("expired token cleanup deletes = %d, want 1", deleted)
	}
}

func TestRefreshService_Revoke(t *testing.T) {
	var deletedHash string
	store := fakeRefreshStore{
		deleteFn: func(ctx context.Context, hash string) error {
			deletedHash = hash
			return nil
		},
	}
	svc := NewRefreshService(store, time.Hour)

	if err := svc.Revoke(context.Background(), "logout-me"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if deletedHash != hashToken("logout-me") {
		t.Error("Revoke deleted the wrong token hash")
	}
}
