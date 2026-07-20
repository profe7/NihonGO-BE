package kana

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
	quizPath   = "/katakana/quiz"
	answerPath = "/katakana/quiz/answer"
	statsPath  = "/katakana/stats"
)

type fakeStore struct {
	randomFn func(
		ctx context.Context,
		characters []string,
	) (Card, error)

	randomOthersFn func(
		ctx context.Context,
		excludeID int64,
		n int,
		characters []string,
	) ([]Card, error)

	findByIDFn func(
		ctx context.Context,
		id int64,
	) (Card, error)

	recordAttemptFn func(
		ctx context.Context,
		userID int64,
		cardID int64,
		correct bool,
	) error

	progressFn func(
		ctx context.Context,
		userID int64,
	) (study.Progress, error)
}

var _ Store = fakeStore{}

func (f fakeStore) Random(
	ctx context.Context,
	characters []string,
) (Card, error) {
	return f.randomFn(ctx, characters)
}

func (f fakeStore) RandomOthers(
	ctx context.Context,
	excludeID int64,
	n int,
	characters []string,
) ([]Card, error) {
	return f.randomOthersFn(ctx, excludeID, n, characters)
}

func (f fakeStore) FindByID(
	ctx context.Context,
	id int64,
) (Card, error) {
	return f.findByIDFn(ctx, id)
}

func (f fakeStore) RecordAttempt(
	ctx context.Context,
	userID int64,
	cardID int64,
	correct bool,
) error {
	return f.recordAttemptFn(ctx, userID, cardID, correct)
}

func (f fakeStore) Progress(
	ctx context.Context,
	userID int64,
) (study.Progress, error) {
	return f.progressFn(ctx, userID)
}

func setup(store Store) *gin.Engine {
	return setupScript(store, Katakana)
}

func setupScript(store Store, script Script) *gin.Engine {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(store, script)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", int64(1))
		c.Next()
	})
	router.GET(quizPath, handler.Quiz)
	router.POST(answerPath, handler.Answer)
	router.GET(statsPath, handler.Stats)

	return router
}

func doJSON(
	router *gin.Engine,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder
}

func TestQuiz(t *testing.T) {
	store := fakeStore{
		randomFn: func(
			ctx context.Context,
			characters []string,
		) (Card, error) {
			return Card{
				ID:        1,
				Character: "ア",
				Romaji:    "a",
			}, nil
		},
		randomOthersFn: func(
			ctx context.Context,
			excludeID int64,
			n int,
			characters []string,
		) ([]Card, error) {
			if excludeID != 1 {
				t.Errorf("excludeID = %d, want 1", excludeID)
			}
			if n != 3 {
				t.Errorf("n = %d, want 3", n)
			}

			return []Card{
				{ID: 2, Character: "イ", Romaji: "i"},
				{ID: 3, Character: "ウ", Romaji: "u"},
				{ID: 4, Character: "エ", Romaji: "e"},
			}, nil
		},
	}

	router := setup(store)
	response := doJSON(
		router,
		http.MethodGet,
		quizPath,
		"",
	)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want 200; body=%s",
			response.Code,
			response.Body.String(),
		)
	}

	var body struct {
		ID        int64    `json:"id"`
		Character string   `json:"character"`
		Options   []string `json:"options"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.ID != 1 {
		t.Errorf("ID = %d, want 1", body.ID)
	}
	if body.Character != "ア" {
		t.Errorf("Character = %q, want ア", body.Character)
	}
	if len(body.Options) != 4 {
		t.Fatalf("option count = %d, want 4", len(body.Options))
	}

	for _, expected := range []string{"a", "i", "u", "e"} {
		if !slices.Contains(body.Options, expected) {
			t.Errorf("options = %v, missing %q", body.Options, expected)
		}
	}
}

func TestQuiz_CharacterFilter(t *testing.T) {
	var randomFilter []string
	var distractorFilter []string

	store := fakeStore{
		randomFn: func(
			ctx context.Context,
			characters []string,
		) (Card, error) {
			randomFilter = characters
			return Card{
				ID:        1,
				Character: "ア",
				Romaji:    "a",
			}, nil
		},
		randomOthersFn: func(
			ctx context.Context,
			excludeID int64,
			n int,
			characters []string,
		) ([]Card, error) {
			distractorFilter = characters
			return []Card{
				{ID: 2, Character: "イ", Romaji: "i"},
			}, nil
		},
	}

	router := setup(store)
	response := doJSON(
		router,
		http.MethodGet,
		quizPath+"?characters=ア,イ,ウ",
		"",
	)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want 200; body=%s",
			response.Code,
			response.Body.String(),
		)
	}

	expected := []string{"ア", "イ", "ウ"}

	if !slices.Equal(randomFilter, expected) {
		t.Errorf(
			"Random filter = %v, want %v",
			randomFilter,
			expected,
		)
	}
	if !slices.Equal(distractorFilter, expected) {
		t.Errorf(
			"RandomOthers filter = %v, want %v",
			distractorFilter,
			expected,
		)
	}
}

func TestQuiz_Errors(t *testing.T) {
	tests := []struct {
		name       string
		store      fakeStore
		wantStatus int
	}{
		{
			name: "no matching cards",
			store: fakeStore{
				randomFn: func(
					ctx context.Context,
					characters []string,
				) (Card, error) {
					return Card{}, ErrNotFound
				},
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "random card failure",
			store: fakeStore{
				randomFn: func(
					ctx context.Context,
					characters []string,
				) (Card, error) {
					return Card{}, errors.New("db down")
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "distractor failure",
			store: fakeStore{
				randomFn: func(
					ctx context.Context,
					characters []string,
				) (Card, error) {
					return Card{
						ID:        1,
						Character: "ア",
						Romaji:    "a",
					}, nil
				},
				randomOthersFn: func(
					ctx context.Context,
					excludeID int64,
					n int,
					characters []string,
				) ([]Card, error) {
					return nil, errors.New("db down")
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := setup(test.store)
			response := doJSON(
				router,
				http.MethodGet,
				quizPath,
				"",
			)

			if response.Code != test.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body=%s",
					response.Code,
					test.wantStatus,
					response.Body.String(),
				)
			}
		})
	}
}

func TestQuiz_NoMatchUsesScriptName(t *testing.T) {
	tests := []struct {
		name      string
		script    Script
		wantError string
	}{
		{
			name:      "hiragana",
			script:    Hiragana,
			wantError: "no hiragana cards match the given characters",
		},
		{
			name:      "katakana",
			script:    Katakana,
			wantError: "no katakana cards match the given characters",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := fakeStore{
				randomFn: func(
					ctx context.Context,
					characters []string,
				) (Card, error) {
					return Card{}, ErrNotFound
				},
			}

			router := setupScript(store, test.script)
			response := doJSON(router, http.MethodGet, quizPath, "")
			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"status = %d, want 400; body=%s",
					response.Code,
					response.Body.String(),
				)
			}

			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Error != test.wantError {
				t.Errorf("error = %q, want %q", body.Error, test.wantError)
			}
		})
	}
}

func TestAnswer_Correct(t *testing.T) {
	var gotUserID int64
	var gotCardID int64
	var gotCorrect bool

	store := fakeStore{
		findByIDFn: func(
			ctx context.Context,
			id int64,
		) (Card, error) {
			if id != 7 {
				t.Errorf("FindByID ID = %d, want 7", id)
			}

			return Card{
				ID:        7,
				Character: "ア",
				Romaji:    "a",
			}, nil
		},
		recordAttemptFn: func(
			ctx context.Context,
			userID int64,
			cardID int64,
			correct bool,
		) error {
			gotUserID = userID
			gotCardID = cardID
			gotCorrect = correct
			return nil
		},
	}

	router := setup(store)
	response := doJSON(
		router,
		http.MethodPost,
		answerPath,
		`{"id":7,"answer":" A "}`,
	)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want 200; body=%s",
			response.Code,
			response.Body.String(),
		)
	}

	if gotUserID != 1 {
		t.Errorf("userID = %d, want 1", gotUserID)
	}
	if gotCardID != 7 {
		t.Errorf("cardID = %d, want 7", gotCardID)
	}
	if !gotCorrect {
		t.Error("correct = false, want true")
	}

	var body struct {
		Correct       bool   `json:"correct"`
		CorrectAnswer string `json:"correct_answer"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !body.Correct {
		t.Error("response correct = false, want true")
	}
	if body.CorrectAnswer != "a" {
		t.Errorf(
			"correct_answer = %q, want a",
			body.CorrectAnswer,
		)
	}
}

func TestAnswer_Incorrect(t *testing.T) {
	var recordedCorrect bool
	var attemptRecorded bool

	store := fakeStore{
		findByIDFn: func(
			ctx context.Context,
			id int64,
		) (Card, error) {
			return Card{
				ID:        id,
				Character: "ア",
				Romaji:    "a",
			}, nil
		},
		recordAttemptFn: func(
			ctx context.Context,
			userID int64,
			cardID int64,
			correct bool,
		) error {
			recordedCorrect = correct
			attemptRecorded = true
			return nil
		},
	}

	router := setup(store)
	response := doJSON(
		router,
		http.MethodPost,
		answerPath,
		`{"id":7,"answer":"i"}`,
	)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want 200; body=%s",
			response.Code,
			response.Body.String(),
		)
	}
	if !attemptRecorded {
		t.Fatal("attempt was not recorded")
	}
	if recordedCorrect {
		t.Error("recorded correct = true, want false")
	}

	var body struct {
		Correct       bool   `json:"correct"`
		CorrectAnswer string `json:"correct_answer"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Correct {
		t.Error("response correct = true, want false")
	}
	if body.CorrectAnswer != "a" {
		t.Errorf(
			"correct_answer = %q, want a",
			body.CorrectAnswer,
		)
	}
}

func TestAnswer_Errors(t *testing.T) {
	tests := []struct {
		name       string
		store      fakeStore
		body       string
		wantStatus int
	}{
		{
			name:       "invalid request",
			store:      fakeStore{},
			body:       `{"id":7}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "unknown question",
			store: fakeStore{
				findByIDFn: func(
					ctx context.Context,
					id int64,
				) (Card, error) {
					return Card{}, ErrNotFound
				},
			},
			body:       `{"id":999,"answer":"a"}`,
			wantStatus: http.StatusNotFound,
		},
		{
			name: "find card failure",
			store: fakeStore{
				findByIDFn: func(
					ctx context.Context,
					id int64,
				) (Card, error) {
					return Card{}, errors.New("db down")
				},
			},
			body:       `{"id":7,"answer":"a"}`,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "record attempt failure",
			store: fakeStore{
				findByIDFn: func(
					ctx context.Context,
					id int64,
				) (Card, error) {
					return Card{
						ID:        id,
						Character: "ア",
						Romaji:    "a",
					}, nil
				},
				recordAttemptFn: func(
					ctx context.Context,
					userID int64,
					cardID int64,
					correct bool,
				) error {
					return errors.New("db down")
				},
			},
			body:       `{"id":7,"answer":"a"}`,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := setup(test.store)
			response := doJSON(
				router,
				http.MethodPost,
				answerPath,
				test.body,
			)

			if response.Code != test.wantStatus {
				t.Fatalf(
					"status = %d, want %d; body=%s",
					response.Code,
					test.wantStatus,
					response.Body.String(),
				)
			}
		})
	}
}

func TestAnswer_Unauthorized(t *testing.T) {
	store := fakeStore{}
	handler := NewHandler(store, Katakana)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(answerPath, handler.Answer)

	response := doJSON(
		router,
		http.MethodPost,
		answerPath,
		`{"id":7,"answer":"a"}`,
	)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf(
			"status = %d, want 401; body=%s",
			response.Code,
			response.Body.String(),
		)
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

	router := setup(store)
	response := doJSON(
		router,
		http.MethodGet,
		statsPath,
		"",
	)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want 200; body=%s",
			response.Code,
			response.Body.String(),
		)
	}
	if gotUserID != 1 {
		t.Errorf("userID = %d, want 1", gotUserID)
	}

	var body statsResponse
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.TotalAttempts != 3 {
		t.Errorf(
			"TotalAttempts = %d, want 3",
			body.TotalAttempts,
		)
	}
	if body.CorrectAttempts != 2 {
		t.Errorf(
			"CorrectAttempts = %d, want 2",
			body.CorrectAttempts,
		)
	}

	expectedAccuracy := (study.Progress{
		TotalAttempts:   3,
		CorrectAttempts: 2,
	}).AccuracyPercent()

	if body.AccuracyPercent != expectedAccuracy {
		t.Errorf(
			"AccuracyPercent = %v, want %v",
			body.AccuracyPercent,
			expectedAccuracy,
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

	router := setup(store)
	response := doJSON(
		router,
		http.MethodGet,
		statsPath,
		"",
	)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf(
			"status = %d, want 500; body=%s",
			response.Code,
			response.Body.String(),
		)
	}
}

func TestStats_Unauthorized(t *testing.T) {
	store := fakeStore{}
	handler := NewHandler(store, Katakana)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET(statsPath, handler.Stats)

	response := doJSON(
		router,
		http.MethodGet,
		statsPath,
		"",
	)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf(
			"status = %d, want 401; body=%s",
			response.Code,
			response.Body.String(),
		)
	}
}
