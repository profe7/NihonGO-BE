package katakana

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nihongo/internal/study"
)

var ErrNotFound = errors.New("katakana card not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Random(
	ctx context.Context,
	characters []string,
) (Card, error) {
	const query = `
			SELECT id, character, romaji
			FROM katakana
			WHERE $1::text[] IS NULL OR character = ANY($1)
			ORDER BY random()
			LIMIT 1`

	var card Card
	err := r.pool.QueryRow(ctx, query, characters).Scan(
		&card.ID,
		&card.Character,
		&card.Romaji,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Card{}, ErrNotFound
		}
		return Card{}, fmt.Errorf("select random katakana card: %w", err)
	}

	return card, nil
}

func (r *Repository) RandomOthers(
	ctx context.Context,
	excludeID int64,
	n int,
	characters []string,
) ([]Card, error) {
	const query = `
                SELECT id, character, romaji
                FROM katakana
                WHERE id != $1
                  AND ($3::text[] IS NULL OR character = ANY($3))
                ORDER BY random()
                LIMIT $2`

	rows, err := r.pool.Query(ctx, query, excludeID, n, characters)
	if err != nil {
		return nil, fmt.Errorf("select katakana distractors: %w", err)
	}
	defer rows.Close()

	cards := make([]Card, 0, n)
	for rows.Next() {
		var card Card
		if err := rows.Scan(
			&card.ID,
			&card.Character,
			&card.Romaji,
		); err != nil {
			return nil, fmt.Errorf("scan katakana distractors: %w", err)
		}
		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate katakana distractors: %w", err)
	}

	return cards, nil
}

func (r *Repository) FindByID(
	ctx context.Context,
	id int64,
) (Card, error) {
	const query = `
                SELECT id, character, romaji
                FROM katakana
                WHERE id = $1`

	var card Card
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&card.ID,
		&card.Character,
		&card.Romaji,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Card{}, ErrNotFound
		}
		return Card{}, fmt.Errorf(
			"find katakana card %d: %w",
			id,
			err,
		)
	}

	return card, nil
}

func (r *Repository) RecordAttempt(
	ctx context.Context,
	userID int64,
	cardID int64,
	correct bool,
) error {
	const query = `
                INSERT INTO katakana_attempts (user_id, card_id, correct)
                VALUES ($1, $2, $3)`

	if _, err := r.pool.Exec(
		ctx,
		query,
		userID,
		cardID,
		correct,
	); err != nil {
		return fmt.Errorf(
			"record katakana attempt for user %d and card %d: %w",
			userID,
			cardID,
			err,
		)
	}

	return nil
}

func (r *Repository) Progress(
	ctx context.Context,
	userID int64,
) (study.Progress, error) {
	const query = `
                SELECT
                        COUNT(*),
                        COUNT(*) FILTER (WHERE correct)
                FROM katakana_attempts
                WHERE user_id = $1`

	var progress study.Progress
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&progress.TotalAttempts,
		&progress.CorrectAttempts,
	)
	if err != nil {
		return study.Progress{}, fmt.Errorf(
			"load katakana progress for user %d: %w",
			userID,
			err,
		)
	}

	return progress, nil
}
