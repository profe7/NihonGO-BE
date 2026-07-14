package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"nihongo/internal/user"
)

type fakeStore struct {
	createFn      func(ctx context.Context, email, hash string) (user.User, error)
	findByEmailFn func(ctx context.Context, email string) (user.User, error)
	findByIDFn    func(ctx context.Context, id int64) (user.User, error)
}

func (f fakeStore) Create(ctx context.Context, email, hash string) (user.User, error) {
	return f.createFn(ctx, email, hash)
}

func (f fakeStore) FindByEmail(ctx context.Context, email string) (user.User, error) {
	return f.findByEmailFn(ctx, email)
}

func (f fakeStore) FindByID(ctx context.Context, id int64) (user.User, error) {
	return f.findByIDFn(ctx, id)
}

type fakeRefreshStore struct {
	createFn     func(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error
	findByHashFn func(ctx context.Context, tokenHash string) (user.RefreshToken, error)
	deleteFn     func(ctx context.Context, tokenHash string) error
}

func (f fakeRefreshStore) Create(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	if f.createFn == nil {
		return nil
	}
	return f.createFn(ctx, userID, tokenHash, expiresAt)
}

func (f fakeRefreshStore) FindByHash(ctx context.Context, tokenHash string) (user.RefreshToken, error) {
	if f.findByHashFn == nil {
		return user.RefreshToken{}, user.ErrNotFound
	}
	return f.findByHashFn(ctx, tokenHash)
}

func (f fakeRefreshStore) Delete(ctx context.Context, tokenHash string) error {
	if f.deleteFn == nil {
		return nil
	}
	return f.deleteFn(ctx, tokenHash)
}

func setup(store UserStore) (*gin.Engine, *TokenService) {
	return setupWithRefresh(store, fakeRefreshStore{})
}

func setupWithRefresh(store UserStore, rstore RefreshStore) (*gin.Engine, *TokenService) {
	gin.SetMode(gin.TestMode)
	tokens := NewTokenService("test-secret", time.Hour)
	refresh := NewRefreshService(rstore, time.Hour)
	h := NewHandler(store, tokens, refresh)
	r := gin.New()
	r.POST("/auth/register", h.Register)
	r.POST("/auth/login", h.Login)
	r.POST("/auth/refresh", h.Refresh)
	r.POST("/auth/logout", h.Logout)
	r.GET("/me", RequireAuth(tokens), h.Me)
	return r, tokens
}

func doJSON(r *gin.Engine, method, path, body, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRegister(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		createErr  error
		wantStatus int
	}{
		{"success", `{"email":"a@b.com","password":"password1"}`, nil, http.StatusCreated},
		{"invalid email", `{"email":"nope","password":"password1"}`, nil, http.StatusBadRequest},
		{"short password", `{"email":"a@b.com","password":"short"}`, nil, http.StatusBadRequest},
		{"duplicate", `{"email":"a@b.com","password":"password1"}`, user.ErrEmailTaken, http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := fakeStore{
				createFn: func(ctx context.Context, email, hash string) (user.User, error) {
					if tc.createErr != nil {
						return user.User{}, tc.createErr
					}
					return user.User{ID: 1, Email: email}, nil
				},
			}
			r, _ := setup(store)

			w := doJSON(r, http.MethodPost, "/auth/register", tc.body, "")

			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}
}

func TestLogin(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password1"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	stored := user.User{ID: 7, Email: "a@b.com", PasswordHash: string(hash)}

	t.Run("success returns a token", func(t *testing.T) {
		store := fakeStore{
			findByEmailFn: func(ctx context.Context, email string) (user.User, error) {
				return stored, nil
			},
		}
		r, tokens := setup(store)

		w := doJSON(r, http.MethodPost, "/auth/login", `{"email":"a@b.com","password":"password1"}`, "")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}

		var resp struct {
			Token string `json:"access_token"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		id, err := tokens.Verify(resp.Token)
		if err != nil {
			t.Fatalf("returned token failed verification: %v", err)
		}
		if id != 7 {
			t.Errorf("token subject = %d, want 7", id)
		}
	})

	t.Run("wrong password is 401", func(t *testing.T) {
		store := fakeStore{
			findByEmailFn: func(ctx context.Context, email string) (user.User, error) {
				return stored, nil
			},
		}
		r, _ := setup(store)

		w := doJSON(r, http.MethodPost, "/auth/login", `{"email":"a@b.com","password":"wrongpass"}`, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})

	t.Run("unknown email is 401", func(t *testing.T) {
		store := fakeStore{
			findByEmailFn: func(ctx context.Context, email string) (user.User, error) {
				return user.User{}, user.ErrNotFound
			},
		}
		r, _ := setup(store)

		w := doJSON(r, http.MethodPost, "/auth/login", `{"email":"ghost@b.com","password":"password1"}`, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})
}

func TestMe(t *testing.T) {
	store := fakeStore{
		findByIDFn: func(ctx context.Context, id int64) (user.User, error) {
			return user.User{ID: id, Email: "a@b.com"}, nil
		},
	}
	r, tokens := setup(store)

	t.Run("no token is 401", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/me", "", "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})

	t.Run("valid token returns the user", func(t *testing.T) {
		token, err := tokens.Generate(7)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		w := doJSON(r, http.MethodGet, "/me", "", token)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}

		var got user.User
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.ID != 7 {
			t.Errorf("id = %d, want 7", got.ID)
		}
	})
}

func TestRefresh(t *testing.T) {
	t.Run("valid token rotates and returns a new pair", func(t *testing.T) {
		var deleted, created int
		rstore := fakeRefreshStore{
			findByHashFn: func(ctx context.Context, hash string) (user.RefreshToken, error) {
				return user.RefreshToken{UserID: 4, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)}, nil
			},
			deleteFn: func(ctx context.Context, hash string) error { deleted++; return nil },
			createFn: func(ctx context.Context, userID int64, hash string, exp time.Time) error { created++; return nil },
		}
		r, tokens := setupWithRefresh(fakeStore{}, rstore)

		w := doJSON(r, http.MethodPost, "/auth/refresh", `{"refresh_token":"old-raw"}`, "")
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
		}

		var resp struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		id, err := tokens.Verify(resp.AccessToken)
		if err != nil {
			t.Fatalf("returned access token failed verification: %v", err)
		}
		if id != 4 {
			t.Errorf("access token subject = %d, want 4", id)
		}
		if resp.RefreshToken == "" {
			t.Error("expected a rotated refresh token")
		}
		if deleted != 1 {
			t.Errorf("old token deletions = %d, want 1", deleted)
		}
		if created != 1 {
			t.Errorf("new token creations = %d, want 1", created)
		}
	})

	t.Run("unknown token is 401", func(t *testing.T) {
		rstore := fakeRefreshStore{
			findByHashFn: func(ctx context.Context, hash string) (user.RefreshToken, error) {
				return user.RefreshToken{}, user.ErrNotFound
			},
		}
		r, _ := setupWithRefresh(fakeStore{}, rstore)

		w := doJSON(r, http.MethodPost, "/auth/refresh", `{"refresh_token":"nope"}`, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})

	t.Run("missing field is 400", func(t *testing.T) {
		r, _ := setup(fakeStore{})
		w := doJSON(r, http.MethodPost, "/auth/refresh", `{}`, "")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}

func TestLogout(t *testing.T) {
	t.Run("revokes and returns 204", func(t *testing.T) {
		var deletedHash string
		rstore := fakeRefreshStore{
			deleteFn: func(ctx context.Context, hash string) error {
				deletedHash = hash
				return nil
			},
		}
		r, _ := setupWithRefresh(fakeStore{}, rstore)

		w := doJSON(r, http.MethodPost, "/auth/logout", `{"refresh_token":"bye"}`, "")
		if w.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", w.Code)
		}
		if deletedHash == "" {
			t.Error("expected logout to delete a token hash")
		}
	})

	t.Run("missing field is 400", func(t *testing.T) {
		r, _ := setup(fakeStore{})
		w := doJSON(r, http.MethodPost, "/auth/logout", `{}`, "")
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}
