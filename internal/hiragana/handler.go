package hiragana

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Store interface {
	Random(ctx context.Context) (Card, error)
	RandomOthers(ctx context.Context, excludeID int64, n int) ([]Card, error)
	FindByID(ctx context.Context, id int64) (Card, error)
}

type Handler struct {
	cards Store
}

func NewHandler(cards Store) *Handler {
	return &Handler{cards: cards}
}

func (h *Handler) Quiz(c *gin.Context) {
	target, err := h.cards.Random(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not build quiz"})
		return
	}

	distractors, err := h.cards.RandomOthers(c.Request.Context(), target.ID, 3)
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
	c.JSON(http.StatusOK, gin.H{
		"correct":        correct,
		"correct_answer": card.Romaji,
	})
}
