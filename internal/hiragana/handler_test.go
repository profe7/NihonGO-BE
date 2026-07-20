package hiragana

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"nihongo/internal/study"
)

const (
	quizPath   = "/hiragana/quiz"
	answerPath = "/hiragana/quiz/answer"
	statsPath  = "/hiragana/stats"
)

type fakeStore struct {
	randomFn        func(ctx context.Context, characters []string) (Card, error)
	randomOthersFn  func(ctx context.Context, excludeID int64, n int, characters []string) ([]Card, error)
	findByIDFn      func(ctx context.Context, id int64) (Card, error)
	recordAttemptFn func(ctx context.Context, userID, cardID int64, correct bool) error
	progressFn      func(ctx context.Context, userID int64) (study.Progress, error)
}

func (f fakeStore) Random(ctx context.Context, characters []string) (Card, error) {
	return f.randomFn(ctx, characters)
}

func (f fakeStore) RandomOthers(ctx context.Context, excludeID int64, n int, characters []string) ([]Card, error) {
	return f.randomOthersFn(ctx, excludeID, n, characters)
}

func (f fakeStore) FindByID(ctx context.Context, id int64) (Card, error) {
	return f.findByIDFn(ctx, id)
}

func (f fakeStore) RecordAttempt(ctx context.Context, userID, cardID int64, correct bool) error {
	return f.recordAttemptFn(ctx, userID, cardID, correct)
}

func (f fakeStore) Progress(
	ctx context.Context,
	userID int64,
) (study.Progress, error) {
	return f.progressFn(ctx, userID)
}

func setup(store Store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(store)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", int64(1))
		c.Next()
	})
	r.GET(quizPath, h.Quiz)
	r.POST(answerPath, h.Answer)
	r.GET(statsPath, h.Stats)
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
		randomFn: func(ctx context.Context, characters []string) (Card, error) {
			return Card{ID: 1, Character: "あ", Romaji: "a"}, nil
		},
		randomOthersFn: func(ctx context.Context, excludeID int64, n int, characters []string) ([]Card, error) {
			return []Card{
				{ID: 2, Character: "い", Romaji: "i"},
				{ID: 3, Character: "う", Romaji: "u"},
				{ID: 4, Character: "え", Romaji: "e"},
			}, nil
		},
	}
	r := setup(store)

	w := doJSON(r, http.MethodGet, quizPath, "")
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

func TestQuiz_CharacterPool(t *testing.T) {
	var gotRandomPool []string
	var gotOthersPool []string

	store := fakeStore{
		randomFn: func(ctx context.Context, characters []string) (Card, error) {
			gotRandomPool = characters
			return Card{ID: 1, Character: "あ", Romaji: "a"}, nil
		},
		randomOthersFn: func(ctx context.Context, excludeID int64, n int, characters []string) ([]Card, error) {
			gotOthersPool = characters
			return []Card{{ID: 2, Character: "い", Romaji: "i"}}, nil
		},
	}
	r := setup(store)

	w := doJSON(r, http.MethodGet, quizPath+"?characters=あ,い,う", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	want := []string{"あ", "い", "う"}
	if !slices.Equal(gotRandomPool, want) {
		t.Errorf("Random pool = %v, want %v", gotRandomPool, want)
	}
	if !slices.Equal(gotOthersPool, want) {
		t.Errorf("RandomOthers pool = %v, want %v", gotOthersPool, want)
	}
}

func TestQuiz_NoCharactersParamMeansNilPool(t *testing.T) {
	var gotPool []string
	sawCall := false

	store := fakeStore{
		randomFn: func(ctx context.Context, characters []string) (Card, error) {
			gotPool = characters
			sawCall = true
			return Card{ID: 1, Character: "あ", Romaji: "a"}, nil
		},
		randomOthersFn: func(ctx context.Context, excludeID int64, n int, characters []string) ([]Card, error) {
			return nil, nil
		},
	}
	r := setup(store)

	doJSON(r, http.MethodGet, quizPath, "")

	if !sawCall {
		t.Fatal("Random was never called")
	}
	if gotPool != nil {
		t.Errorf("pool = %v, want nil (no filter) when characters param is absent", gotPool)
	}
}

func TestQuiz_NoMatchingCharactersIs400(t *testing.T) {
	store := fakeStore{
		randomFn: func(ctx context.Context, characters []string) (Card, error) {
			return Card{}, ErrNotFound
		},
	}
	r := setup(store)

	w := doJSON(r, http.MethodGet, quizPath+"?characters=ZZZ", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
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
		recordAttemptFn: func(ctx context.Context, userID, cardID int64, correct bool) error {
			return nil
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
			w := doJSON(r, http.MethodPost, answerPath, tc.body)
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
		w := doJSON(r, http.MethodPost, answerPath, `{"id":99,"answer":"na"}`)
		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})

	t.Run("missing field is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, answerPath, `{}`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})
}

func TestAnswer_RecordAttempt(t *testing.T) {
	var gotUserID, gotCardID int64
	var gotCorrect, gotCalled bool

	store := fakeStore{
		findByIDFn: func(ctx context.Context, id int64) (Card, error) {
			if id == 99 {
				return Card{}, ErrNotFound
			}
			return Card{ID: id, Character: "な", Romaji: "na"}, nil
		},
		recordAttemptFn: func(ctx context.Context, userID, cardID int64, correct bool) error {
			gotUserID = userID
			gotCardID = cardID
			gotCorrect = correct
			gotCalled = true
			return nil
		},
	}

	r := setup(store)

	w := doJSON(r, http.MethodPost, answerPath, `{"id":1,"answer":"na"}`)

	if gotUserID != 1 {
		t.Fatalf("userID = %d, want 1; body=%s", gotUserID, w.Body.String())
	}
	if gotCardID != 1 {
		t.Fatalf("cardID = %d, want 1; body=%s", gotCardID, w.Body.String())
	}
	if !gotCorrect {
		t.Fatalf("correct = %t, want true; body=%s", gotCorrect, w.Body.String())
	}
	if !gotCalled {
		t.Fatalf("called = %t, want true; body=%s", gotCalled, w.Body.String())
	}
}

func TestAnswer_RecordAttemptFailureIs500(t *testing.T) {
	store := fakeStore{
		findByIDFn: func(ctx context.Context, id int64) (Card, error) {
			if id == 99 {
				return Card{}, ErrNotFound
			}
			return Card{ID: id, Character: "な", Romaji: "na"}, nil
		},
		recordAttemptFn: func(ctx context.Context, userID, cardID int64, correct bool) error {
			return errors.New("db down")
		},
	}

	r := setup(store)

	w := doJSON(r, http.MethodPost, answerPath, `{"id":1,"answer":"na"}`)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("error = %d, want 500; body=%s", w.Code, w.Body.String())
	}
}

func TestAnswer_FindByIDFailureIs500(t *testing.T) {
	store := fakeStore{
		findByIDFn: func(ctx context.Context, id int64) (Card, error) {
			return Card{}, errors.New("internal server error")
		},
		recordAttemptFn: func(ctx context.Context, userID, cardID int64, correct bool) error {
			return nil
		},
	}

	r := setup(store)

	w := doJSON(r, http.MethodPost, answerPath, `{"id":1,"answer":"na"}`)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("error = %d, want 500; body=%s", w.Code, w.Body.String())
	}
}

func TestAnswer_Unauthorized(t *testing.T) {
	store := fakeStore{}

	h := NewHandler(store)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST(answerPath, h.Answer)

	w := doJSON(r, http.MethodPost, answerPath, `{"id":1,"answer":"na"}`)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("error = %d, want 401; body=%s", w.Code, w.Body.String())
	}
}

func TestAnswer_RecordAttemptIncorrectAnswer(t *testing.T) {
	var gotCorrect, gotCalled bool

	store := fakeStore{
		findByIDFn: func(ctx context.Context, id int64) (Card, error) {
			return Card{ID: 1, Character: "な", Romaji: "na"}, nil
		},
		recordAttemptFn: func(ctx context.Context, userID, cardID int64, correct bool) error {
			gotCorrect = correct
			gotCalled = true
			return nil
		},
	}

	r := setup(store)

	w := doJSON(r, http.MethodPost, answerPath, `{"id":2,"answer":"ka"}`)

	if gotCorrect {
		t.Fatalf("correct = %t, want false; body=%s", gotCorrect, w.Body.String())
	}
	if !gotCalled {
		t.Fatalf("called = %t, want true; body=%s", gotCalled, w.Body.String())
	}
}

func TestParseCharacters(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty", "", nil},
		{"single character", "あ", []string{"あ"}},
		{"trims whitespace", " あ, い ,う ", []string{"あ", "い", "う"}},
		{"ignores empty entries", "あ,, ,い", []string{"あ", "い"}},
		{"only separators", ", ,", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCharacters(tc.raw)

			if tc.want == nil {
				if got != nil {
					t.Fatalf("parseCharacters(%q) = %v, want nil", tc.raw, got)
				}
				return
			}

			if !slices.Equal(got, tc.want) {
				t.Fatalf("parseCharacters(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestStats(t *testing.T) {
	var gotUserID int64

	store := fakeStore{
		progressFn: func(
			ctx context.Context,
			userID int64,
		) (study.Progress, error) {
			gotUserID = userID
			return study.Progress{
				TotalAttempts:   3,
				CorrectAttempts: 2,
			}, nil
		},
	}

	r := setup(store)
	w := doJSON(r, http.MethodGet, statsPath, "")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if gotUserID != 1 {
		t.Errorf("userID = %d, want 1", gotUserID)
	}

	var response statsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.TotalAttempts != 3 {
		t.Errorf("TotalAttempts = %d, want 3", response.TotalAttempts)
	}
	if response.CorrectAttempts != 2 {
		t.Errorf("CorrectAttempts = %d, want 2", response.CorrectAttempts)
	}

	wantAccuracy := (study.Progress{
		TotalAttempts:   3,
		CorrectAttempts: 2,
	}).AccuracyPercent()

	if response.AccuracyPercent != wantAccuracy {
		t.Errorf(
			"AccuracyPercent = %v, want %v",
			response.AccuracyPercent,
			wantAccuracy,
		)
	}
}

func TestStats_ProgressFailureIs500(t *testing.T) {
	store := fakeStore{
		progressFn: func(
			ctx context.Context,
			userID int64,
		) (study.Progress, error) {
			return study.Progress{}, errors.New("db down")
		},
	}

	r := setup(store)
	w := doJSON(r, http.MethodGet, statsPath, "")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want 500; body=%s",
			w.Code,
			w.Body.String(),
		)
	}
}

func TestStats_Unauthorized(t *testing.T) {
	store := fakeStore{}

	h := NewHandler(store)
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET(statsPath, h.Stats)

	w := doJSON(r, http.MethodGet, statsPath, "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf(
			"status = %d, want 401; body=%s",
			w.Code,
			w.Body.String(),
		)
	}
}
