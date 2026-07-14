package hiragana

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeStore struct {
	randomFn       func(ctx context.Context) (Card, error)
	randomOthersFn func(ctx context.Context, excludeID int64, n int) ([]Card, error)
	findByIDFn     func(ctx context.Context, id int64) (Card, error)
}

func (f fakeStore) Random(ctx context.Context) (Card, error) {
	return f.randomFn(ctx)
}

func (f fakeStore) RandomOthers(ctx context.Context, excludeID int64, n int) ([]Card, error) {
	return f.randomOthersFn(ctx, excludeID, n)
}

func (f fakeStore) FindByID(ctx context.Context, id int64) (Card, error) {
	return f.findByIDFn(ctx, id)
}

func setup(store Store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(store)
	r := gin.New()
	r.GET("/hiragana/quiz", h.Quiz)
	r.POST("/hiragana/quiz/answer", h.Answer)
	return r
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestQuiz(t *testing.T) {
	store := fakeStore{
		randomFn: func(ctx context.Context) (Card, error) {
			return Card{ID: 1, Character: "あ", Romaji: "a"}, nil
		},
		randomOthersFn: func(ctx context.Context, excludeID int64, n int) ([]Card, error) {
			return []Card{
				{ID: 2, Character: "い", Romaji: "i"},
				{ID: 3, Character: "う", Romaji: "u"},
				{ID: 4, Character: "え", Romaji: "e"},
			}, nil
		},
	}
	r := setup(store)

	w := doJSON(r, http.MethodGet, "/hiragana/quiz", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		ID        int64    `json:"id"`
		Character string   `json:"character"`
		Options   []string `json:"options"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID != 1 || resp.Character != "あ" {
		t.Errorf("got id=%d character=%q, want id=1 character=あ", resp.ID, resp.Character)
	}
	if len(resp.Options) != 4 {
		t.Fatalf("len(options) = %d, want 4", len(resp.Options))
	}
	if !slices.Contains(resp.Options, "a") {
		t.Errorf("options = %v, missing correct answer a", resp.Options)
	}
}

func TestAnswer(t *testing.T) {
	store := fakeStore{
		findByIDFn: func(ctx context.Context, id int64) (Card, error) {
			if id == 99 {
				return Card{}, ErrNotFound
			}
			return Card{ID: id, Character: "な", Romaji: "na"}, nil
		},
	}
	r := setup(store)

	cases := []struct {
		name        string
		body        string
		wantStatus  int
		wantCorrect bool
	}{
		{"correct", `{"id":1,"answer":"na"}`, http.StatusOK, true},
		{"wrong", `{"id":1,"answer":"ma"}`, http.StatusOK, false},
		{"case and whitespace tolerant", `{"id":1,"answer":"  NA  "}`, http.StatusOK, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(r, http.MethodPost, "/hiragana/quiz/answer", tc.body)
			if w.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", w.Code, tc.wantStatus, w.Body.String())
			}
			var resp struct {
				Correct bool `json:"correct"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp.Correct != tc.wantCorrect {
				t.Errorf("correct = %v, want %v", resp.Correct, tc.wantCorrect)
			}
		})
	}

	t.Run("unknown id is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/hiragana/quiz/answer", `{"id":99,"answer":"na"}`)
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})

	t.Run("missing field is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/hiragana/quiz/answer", `{}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}
