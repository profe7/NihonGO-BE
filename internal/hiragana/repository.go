package hiragana

import (
	"context"
	"errors"
	"fmt"

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

func (r *Repository) Random(ctx context.Context, characters []string) (Card, error) {
	const q = `
              SELECT id, character, romaji FROM hiragana
              WHERE $1::text[] IS NULL OR character = ANY($1)
              ORDER BY random()
              LIMIT 1`

	var c Card
	if err := r.pool.QueryRow(ctx, q, characters).Scan(&c.ID, &c.Character, &c.Romaji); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Card{}, ErrNotFound
		}
		return Card{}, err
	}
	return c, nil
}

func (r *Repository) RandomOthers(ctx context.Context, excludeID int64, n int, characters []string) ([]Card, error) {
	const q = `
              SELECT id, character, romaji FROM hiragana
              WHERE id != $1
                AND ($3::text[] IS NULL OR character = ANY($3))
              ORDER BY random()
              LIMIT $2`

	rows, err := r.pool.Query(ctx, q, excludeID, n, characters)
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
	if err := r.pool.QueryRow(ctx, q, id).Scan(&c.ID, &c.Character, &c.Romaji); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Card{}, ErrNotFound
		}
		return Card{}, err
	}
	return c, nil
}

func (r *Repository) RecordAttempt(ctx context.Context, userID, cardID int64, correct bool) error {
	const q = `
                INSERT INTO hiragana_attempts (user_id, card_id, correct)
                VALUES ($1, $2, $3)`
	if _, err := r.pool.Exec(ctx, q, userID, cardID, correct); err != nil {
		return fmt.Errorf("record hiragana attempt: %w", err)
	}
	return nil
}
