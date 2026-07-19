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
	var gotOldHash, gotNewHash string
	var gotExpiry time.Time

	store := fakeRefreshStore{
		rotateFn: func(
			ctx context.Context,
			oldHash, newHash string,
			newExpiresAt time.Time,
		) (int64, error) {
			gotOldHash = oldHash
			gotNewHash = newHash
			gotExpiry = newExpiresAt
			return 9, nil
		},
	}
	svc := NewRefreshService(store, time.Hour)

	newRaw, userID, err := svc.Rotate(context.Background(), "old-raw")
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if userID != 9 {
		t.Fatalf("user ID = %d, want 9", userID)
	}
	if newRaw == "" {
		t.Fatal("expected a new raw token")
	}
	if gotOldHash != hashToken("old-raw") {
		t.Fatal("Rotate passed the wrong old-token hash")
	}
	if gotNewHash != hashToken(newRaw) {
		t.Fatal("Rotate stored a hash that does not match the returned raw token")
	}
	if !gotExpiry.After(time.Now()) {
		t.Fatal("new token expiry is not in the future")
	}
}

func TestRefreshService_RotateRejectsMissing(t *testing.T) {
	store := fakeRefreshStore{
		rotateFn: func(
			ctx context.Context,
			oldHash, newHash string,
			newExpiresAt time.Time,
		) (int64, error) {
			return 0, user.ErrNotFound
		},
	}
	svc := NewRefreshService(store, time.Hour)

	if _, _, err := svc.Rotate(
		context.Background(),
		"whatever",
	); !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("err = %v, want ErrInvalidRefreshToken", err)
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
