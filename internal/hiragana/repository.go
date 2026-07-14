package hiragana

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("hiragana card not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Random(ctx context.Context) (Card, error) {
	const q = `SELECT id, character, romaji FROM hiragana ORDER BY random() LIMIT 1`

	var c Card
	err := r.pool.QueryRow(ctx, q).Scan(&c.ID, &c.Character, &c.Romaji)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Card{}, ErrNotFound
		}
		return Card{}, err
	}
	return c, nil
}

func (r *Repository) RandomOthers(ctx context.Context, excludeID int64, n int) ([]Card, error) {
	const q = `
		SELECT id, character, romaji FROM hiragana
		WHERE id != $1
		ORDER BY random()
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, excludeID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards := make([]Card, 0, n)
	for rows.Next() {
		var c Card
		if err := rows.Scan(&c.ID, &c.Character, &c.Romaji); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

func (r *Repository) FindByID(ctx context.Context, id int64) (Card, error) {
	const q = `SELECT id, character, romaji FROM hiragana WHERE id = $1`

	var c Card
	err := r.pool.QueryRow(ctx, q, id).Scan(&c.ID, &c.Character, &c.Romaji)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Card{}, ErrNotFound
		}
		return Card{}, err
	}
	return c, nil
}
