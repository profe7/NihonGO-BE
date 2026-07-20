package katakana

import (
	"context"
	"errors"
	"os"
	"testing"

	"nihongo/internal/testutil"
	"nihongo/internal/user"
)

func TestRepository_Random(t *testing.T) {
	pool := testutil.OpenDatabase(t, os.DirFS("../.."))
	repository := NewRepository(pool)
	ctx := context.Background()

	t.Run("without filter", func(t *testing.T) {
		card, err := repository.Random(ctx, nil)
		if err != nil {
			t.Fatalf("Random() error: %v", err)
		}
		if card.ID == 0 {
			t.Error("card ID = 0, want a persisted ID")
		}
		if card.Character == "" {
			t.Error("card character is empty")
		}
		if card.Romaji == "" {
			t.Error("card romaji is empty")
		}
	})

	t.Run("with filter", func(t *testing.T) {
		card, err := repository.Random(ctx, []string{"ア"})
		if err != nil {
			t.Fatalf("Random() error: %v", err)
		}
		if card.Character != "ア" {
			t.Errorf("character = %q, want ア", card.Character)
		}
		if card.Romaji != "a" {
			t.Errorf("romaji = %q, want a", card.Romaji)
		}
	})

	t.Run("without match", func(t *testing.T) {
		_, err := repository.Random(ctx, []string{"not-katakana"})
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("Random() error = %v, want ErrNotFound", err)
		}
	})
}

func TestRepository_RandomOthers(t *testing.T) {
	pool := testutil.OpenDatabase(t, os.DirFS("../.."))
	repository := NewRepository(pool)
	ctx := context.Background()

	excluded, err := repository.Random(ctx, []string{"ア"})
	if err != nil {
		t.Fatalf("load excluded card: %v", err)
	}

	t.Run("respects exclusion and limit", func(t *testing.T) {
		cards, err := repository.RandomOthers(
			ctx,
			excluded.ID,
			3,
			nil,
		)
		if err != nil {
			t.Fatalf("RandomOthers() error: %v", err)
		}
		if len(cards) != 3 {
			t.Fatalf("card count = %d, want 3", len(cards))
		}

		seen := map[int64]struct{}{
			excluded.ID: {},
		}

		for _, card := range cards {
			if _, exists := seen[card.ID]; exists {
				t.Errorf("duplicate or excluded card ID: %d", card.ID)
			}
			seen[card.ID] = struct{}{}
		}
	})

	t.Run("respects character filter", func(t *testing.T) {
		cards, err := repository.RandomOthers(
			ctx,
			0,
			10,
			[]string{"イ", "ウ"},
		)
		if err != nil {
			t.Fatalf("RandomOthers() error: %v", err)
		}
		if len(cards) != 2 {
			t.Fatalf("card count = %d, want 2", len(cards))
		}

		allowed := map[string]bool{
			"イ": true,
			"ウ": true,
		}
		for _, card := range cards {
			if !allowed[card.Character] {
				t.Errorf("unexpected character: %q", card.Character)
			}
		}
	})
}

func TestRepository_FindByID(t *testing.T) {
	pool := testutil.OpenDatabase(t, os.DirFS("../.."))
	repository := NewRepository(pool)
	ctx := context.Background()

	expected, err := repository.Random(ctx, []string{"ア"})
	if err != nil {
		t.Fatalf("load expected card: %v", err)
	}

	t.Run("existing ID", func(t *testing.T) {
		card, err := repository.FindByID(ctx, expected.ID)
		if err != nil {
			t.Fatalf("FindByID() error: %v", err)
		}
		if card != expected {
			t.Errorf("card = %+v, want %+v", card, expected)
		}
	})

	t.Run("missing ID", func(t *testing.T) {
		_, err := repository.FindByID(ctx, -1)
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("FindByID() error = %v, want ErrNotFound", err)
		}
	})
}

func TestRepository_RecordAttempt(t *testing.T) {
	pool := testutil.OpenDatabase(t, os.DirFS("../.."))
	repository := NewRepository(pool)
	ctx := context.Background()

	if _, err := pool.Exec(
		ctx,
		`TRUNCATE users RESTART IDENTITY CASCADE`,
	); err != nil {
		t.Fatalf("truncate users: %v", err)
	}

	users := user.NewRepository(pool)
	createdUser, err := users.Create(
		ctx,
		"katakana-attempt@example.com",
		"password-hash",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	card, err := repository.Random(ctx, []string{"ア"})
	if err != nil {
		t.Fatalf("load card: %v", err)
	}

	if err := repository.RecordAttempt(
		ctx,
		createdUser.ID,
		card.ID,
		true,
	); err != nil {
		t.Fatalf("RecordAttempt() error: %v", err)
	}

	var correct bool
	err = pool.QueryRow(ctx, `
                SELECT correct
                FROM katakana_attempts
                WHERE user_id = $1 AND card_id = $2
        `, createdUser.ID, card.ID).Scan(&correct)
	if err != nil {
		t.Fatalf("load recorded attempt: %v", err)
	}
	if !correct {
		t.Error("recorded correct = false, want true")
	}

	if err := repository.RecordAttempt(
		ctx,
		-1,
		card.ID,
		true,
	); err == nil {
		t.Error("RecordAttempt() with unknown user returned nil error")
	}
}

func TestRepository_Progress(t *testing.T) {
	pool := testutil.OpenDatabase(t, os.DirFS("../.."))
	repository := NewRepository(pool)
	ctx := context.Background()

	if _, err := pool.Exec(
		ctx,
		`TRUNCATE users RESTART IDENTITY CASCADE`,
	); err != nil {
		t.Fatalf("truncate users: %v", err)
	}

	users := user.NewRepository(pool)
	createdUser, err := users.Create(
		ctx,
		"katakana-progress@example.com",
		"password-hash",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	progress, err := repository.Progress(ctx, createdUser.ID)
	if err != nil {
		t.Fatalf("load empty progress: %v", err)
	}
	if progress.TotalAttempts != 0 || progress.CorrectAttempts != 0 {
		t.Fatalf("empty progress = %+v, want zero values", progress)
	}

	card, err := repository.Random(ctx, []string{"ア"})
	if err != nil {
		t.Fatalf("load card: %v", err)
	}

	for _, correct := range []bool{true, false, true} {
		if err := repository.RecordAttempt(
			ctx,
			createdUser.ID,
			card.ID,
			correct,
		); err != nil {
			t.Fatalf("record attempt: %v", err)
		}
	}

	progress, err = repository.Progress(ctx, createdUser.ID)
	if err != nil {
		t.Fatalf("load progress: %v", err)
	}
	if progress.TotalAttempts != 3 {
		t.Errorf("TotalAttempts = %d, want 3", progress.TotalAttempts)
	}
	if progress.CorrectAttempts != 2 {
		t.Errorf("CorrectAttempts = %d, want 2", progress.CorrectAttempts)
	}
}
