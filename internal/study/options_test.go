package study

import (
	"slices"
	"testing"
)

func TestBuildOptions(t *testing.T) {
	distractors := []string{"ka", "sa", "ta"}

	options := BuildOptions("a", distractors)

	if len(options) != 4 {
		t.Fatalf("len(options) = %d, want 4", len(options))
	}
	if !slices.Contains(options, "a") {
		t.Errorf("options = %v, missing correct answer %q", options, "a")
	}
	for _, d := range distractors {
		if !slices.Contains(options, d) {
			t.Errorf("options = %v, missing distractor %q", options, d)
		}
	}
}

func TestBuildOptions_DoesNotMutateInput(t *testing.T) {
	distractors := []string{"ka", "sa", "ta"}
	original := slices.Clone(distractors)

	BuildOptions("a", distractors)

	if !slices.Equal(distractors, original) {
		t.Errorf("distractors mutated: got %v, want %v", distractors, original)
	}
}
