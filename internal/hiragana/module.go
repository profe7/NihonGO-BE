package hiragana

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"nihongo/internal/kana"
)

func NewRepository(pool *pgxpool.Pool) *kana.Repository {
	return kana.NewRepository(pool, kana.Hiragana)
}

func NewHandler(cards kana.Store) *kana.Handler {
	return kana.NewHandler(cards, kana.Hiragana)
}
