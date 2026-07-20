package kana

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nihongo/internal/study"
)

var ErrNotFound = errors.New("kana card not found")

type repositoryQueries struct {
	random        string
	randomOthers  string
	findByID      string
	recordAttempt string
	progress      string
}

type Repository struct {
	pool    *pgxpool.Pool
	script  Script
	queries repositoryQueries
}

func NewRepository(pool *pgxpool.Pool, script Script) *Repository {
	return &Repository{
		pool:    pool,
		script:  script,
		queries: buildQueries(script.config()),
	}
}

func buildQueries(config scriptConfig) repositoryQueries {
	return repositoryQueries{
		random: fmt.Sprintf(`
			SELECT id, character, romaji
			FROM %s
			WHERE $1::text[] IS NULL OR character = ANY($1)
			ORDER BY random()
			LIMIT 1`, config.cardsTable),
		randomOthers: fmt.Sprintf(`
			SELECT id, character, romaji
			FROM %s
			WHERE id != $1
			  AND ($3::text[] IS NULL OR character = ANY($3))
			ORDER BY random()
			LIMIT $2`, config.cardsTable),
		findByID: fmt.Sprintf(`
			SELECT id, character, romaji
			FROM %s
			WHERE id = $1`, config.cardsTable),
		recordAttempt: fmt.Sprintf(`
			INSERT INTO %s (user_id, card_id, correct)
			VALUES ($1, $2, $3)`, config.attemptsTable),
		progress: fmt.Sprintf(`
			SELECT
				COUNT(*),
				COUNT(*) FILTER (WHERE correct)
			FROM %s
			WHERE user_id = $1`, config.attemptsTable),
	}
}

func (r *Repository) Random(
	ctx context.Context,
	characters []string,
) (Card, error) {
	var card Card
	err := r.pool.QueryRow(ctx, r.queries.random, characters).Scan(
		&card.ID,
		&card.Character,
		&card.Romaji,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Card{}, ErrNotFound
		}
		return Card{}, fmt.Errorf("select random %s card: %w", r.script, err)
	}

	return card, nil
}

func (r *Repository) RandomOthers(
	ctx context.Context,
	excludeID int64,
	n int,
	characters []string,
) ([]Card, error) {
	rows, err := r.pool.Query(
		ctx,
		r.queries.randomOthers,
		excludeID,
		n,
		characters,
	)
	if err != nil {
		return nil, fmt.Errorf("select %s distractors: %w", r.script, err)
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
			return nil, fmt.Errorf("scan %s distractors: %w", r.script, err)
		}
		cards = append(cards, card)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s distractors: %w", r.script, err)
	}

	return cards, nil
}

func (r *Repository) FindByID(
	ctx context.Context,
	id int64,
) (Card, error) {
	var card Card
	err := r.pool.QueryRow(ctx, r.queries.findByID, id).Scan(
		&card.ID,
		&card.Character,
		&card.Romaji,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Card{}, ErrNotFound
		}
		return Card{}, fmt.Errorf("find %s card %d: %w", r.script, id, err)
	}

	return card, nil
}

func (r *Repository) RecordAttempt(
	ctx context.Context,
	userID int64,
	cardID int64,
	correct bool,
) error {
	if _, err := r.pool.Exec(
		ctx,
		r.queries.recordAttempt,
		userID,
		cardID,
		correct,
	); err != nil {
		return fmt.Errorf(
			"record %s attempt for user %d and card %d: %w",
			r.script,
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
	var progress study.Progress
	err := r.pool.QueryRow(ctx, r.queries.progress, userID).Scan(
		&progress.TotalAttempts,
		&progress.CorrectAttempts,
	)
	if err != nil {
		return study.Progress{}, fmt.Errorf(
			"load %s progress for user %d: %w",
			r.script,
			userID,
			err,
		)
	}

	return progress, nil
}
