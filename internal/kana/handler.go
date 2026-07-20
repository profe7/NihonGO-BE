package kana

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"nihongo/internal/auth"
	"nihongo/internal/study"
)

type Store interface {
	Random(ctx context.Context, characters []string) (Card, error)
	RandomOthers(
		ctx context.Context,
		excludeID int64,
		n int,
		characters []string,
	) ([]Card, error)
	FindByID(ctx context.Context, id int64) (Card, error)
	RecordAttempt(
		ctx context.Context,
		userID int64,
		cardID int64,
		correct bool,
	) error
	Progress(ctx context.Context, userID int64) (study.Progress, error)
}

var _ Store = (*Repository)(nil)

type Handler struct {
	cards  Store
	script Script
}

type answerRequest struct {
	ID     int64  `json:"id" binding:"required"`
	Answer string `json:"answer" binding:"required"`
}

type quizResponse struct {
	ID        int64    `json:"id"`
	Character string   `json:"character"`
	Options   []string `json:"options"`
}

type answerResponse struct {
	Correct       bool   `json:"correct"`
	CorrectAnswer string `json:"correct_answer"`
}

type statsResponse struct {
	TotalAttempts   int64   `json:"total_attempts"`
	CorrectAttempts int64   `json:"correct_attempts"`
	AccuracyPercent float64 `json:"accuracy_percent"`
}

func NewHandler(cards Store, script Script) *Handler {
	_ = script.config()
	return &Handler{cards: cards, script: script}
}

func (h *Handler) Quiz(c *gin.Context) {
	characters := study.ParseCharacters(c.Query("characters"))

	target, err := h.cards.Random(c.Request.Context(), characters)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "no " + string(h.script) + " cards match the given characters",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not build quiz",
		})
		return
	}

	distractors, err := h.cards.RandomOthers(
		c.Request.Context(),
		target.ID,
		3,
		characters,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not build quiz",
		})
		return
	}

	distractorRomaji := make([]string, len(distractors))
	for i, distractor := range distractors {
		distractorRomaji[i] = distractor.Romaji
	}

	c.JSON(http.StatusOK, quizResponse{
		ID:        target.ID,
		Character: target.Character,
		Options:   study.BuildOptions(target.Romaji, distractorRomaji),
	})
}

func (h *Handler) Answer(c *gin.Context) {
	var request answerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	card, err := h.cards.FindByID(c.Request.Context(), request.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown question id"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not check answer",
		})
		return
	}

	correct := strings.EqualFold(
		strings.TrimSpace(request.Answer),
		card.Romaji,
	)

	if err := h.cards.RecordAttempt(
		c.Request.Context(),
		userID,
		card.ID,
		correct,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not record " + string(h.script) + " progress",
		})
		return
	}

	c.JSON(http.StatusOK, answerResponse{
		Correct:       correct,
		CorrectAnswer: card.Romaji,
	})
}

func (h *Handler) Stats(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	progress, err := h.cards.Progress(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not load " + string(h.script) + " statistics",
		})
		return
	}

	c.JSON(http.StatusOK, statsResponse{
		TotalAttempts:   progress.TotalAttempts,
		CorrectAttempts: progress.CorrectAttempts,
		AccuracyPercent: progress.AccuracyPercent(),
	})
}
