package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"nihongo/internal/user"
)

const RefreshTokenTTL = 30 * 24 * time.Hour

var ErrInvalidRefreshToken = errors.New("invalid refresh token")

type RefreshStore interface {
	Create(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error
	Delete(ctx context.Context, tokenHash string) error
	Rotate(
		ctx context.Context,
		oldHash, newHash string,
		newExpiresAt time.Time,
	) (int64, error)
}

type RefreshService struct {
	store RefreshStore
	ttl   time.Duration
}

func NewRefreshService(store RefreshStore, ttl time.Duration) *RefreshService {
	return &RefreshService{store: store, ttl: ttl}
}

func (s *RefreshService) Issue(ctx context.Context, userID int64) (string, error) {
	raw, err := randomToken()
	if err != nil {
		return "", err
	}
	if err := s.store.Create(ctx, userID, hashToken(raw), time.Now().Add(s.ttl)); err != nil {
		return "", err
	}
	return raw, nil
}

func (s *RefreshService) Rotate(
	ctx context.Context,
	rawToken string,
) (string, int64, error) {
	newRaw, err := randomToken()
	if err != nil {
		return "", 0, err
	}

	userID, err := s.store.Rotate(
		ctx,
		hashToken(rawToken),
		hashToken(newRaw),
		time.Now().Add(s.ttl),
	)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return "", 0, ErrInvalidRefreshToken
		}
		return "", 0, err
	}

	return newRaw, userID, nil
}

func (s *RefreshService) Revoke(ctx context.Context, rawToken string) error {
	return s.store.Delete(ctx, hashToken(rawToken))
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
