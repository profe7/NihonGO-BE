package auth

import (
	"testing"
	"time"
)

func TestTokenService_GenerateThenVerify(t *testing.T) {
	svc := NewTokenService("test-secret", time.Hour)

	token, err := svc.Generate(42)
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	userID, err := svc.Verify(token)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if userID != 42 {
		t.Errorf("user id = %d, want 42", userID)
	}
}

func TestTokenService_VerifyRejects(t *testing.T) {
	svc := NewTokenService("test-secret", time.Hour)
	valid, err := svc.Generate(1)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	expiredSvc := NewTokenService("test-secret", -time.Hour)
	expired, err := expiredSvc.Generate(1)
	if err != nil {
		t.Fatalf("Generate expired: %v", err)
	}

	otherSecret := NewTokenService("different-secret", time.Hour)
	wrongSecret, err := otherSecret.Generate(1)
	if err != nil {
		t.Fatalf("Generate wrong-secret: %v", err)
	}

	tampered := flip(valid)

	cases := []struct {
		name  string
		token string
	}{
		{"expired", expired},
		{"signed with different secret", wrongSecret},
		{"tampered payload", tampered},
		{"not a jwt", "not.a.jwt"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Verify(tc.token); err == nil {
				t.Errorf("Verify(%q) = nil error, want an error", tc.name)
			}
		})
	}
}

func flip(token string) string {
	b := []byte(token)
	i := len(b) / 2
	if b[i] == 'a' {
		b[i] = 'b'
	} else {
		b[i] = 'a'
	}
	return string(b)
}
