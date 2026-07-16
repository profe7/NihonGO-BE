package hiragana

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"nihongo/internal/auth"
)

type Store interface {
	Random(ctx context.Context, characters []string) (Card, error)
	RandomOthers(ctx context.Context, excludeID int64, n int, characters []string) ([]Card, error)
	FindByID(ctx context.Context, id int64) (Card, error)
	RecordAttempt(ctx context.Context, userID, cardID int64, correct bool) (error)
}

type Handler struct {
	cards Store
}

func NewHandler(cards Store) *Handler {
	return &Handler{cards: cards}
}

func (h *Handler) Quiz(c *gin.Context) {
	characters := parseCharacters(c.Query("characters"))

	target, err := h.cards.Random(c.Request.Context(), characters)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "no hiragana cards match the given characters"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not build quiz"})
		return
	}

	distractors, err := h.cards.RandomOthers(c.Request.Context(), target.ID, 3, characters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not build quiz"})
		return
	}

	distractorRomaji := make([]string, len(distractors))
	for i, d := range distractors {
		distractorRomaji[i] = d.Romaji
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        target.ID,
		"character": target.Character,
		"options":   BuildOptions(target.Romaji, distractorRomaji),
	})
}

func parseCharacters(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	characters := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			characters = append(characters, p)
		}
	}
	if len(characters) == 0 {
		return nil
	}
	return characters
}

type answerRequest struct {
	ID     int64  `json:"id"     binding:"required"`
	Answer string `json:"answer" binding:"required"`
}

func (h *Handler) Answer(c *gin.Context) {
	var req answerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	card, err := h.cards.FindByID(c.Request.Context(), req.ID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown question id"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not check answer"})
		return
	}

	correct := strings.EqualFold(strings.TrimSpace(req.Answer), card.Romaji)
	err = h.cards.RecordAttempt(c.Request.Context(), userID, card.ID, correct)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error tracking progress"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"correct":        correct,
		"correct_answer": card.Romaji,
	})
}