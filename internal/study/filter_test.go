package study

import (
	"slices"
	"testing"
)

func TestParseCharacters(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"empty", "", nil},
		{"single character", "あ", []string{"あ"}},
		{"trims whitespace", " あ, い ,う ", []string{"あ", "い", "う"}},
		{"ignores empty entries", "あ,, ,い", []string{"あ", "い"}},
		{"only separators", ", ,", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseCharacters(tc.raw)

			if tc.want == nil {
				if got != nil {
					t.Fatalf(
						"ParseCharacters(%q) = %v, want nil",
						tc.raw,
						got,
					)
				}
				return
			}

			if !slices.Equal(got, tc.want) {
				t.Fatalf(
					"ParseCharacters(%q) = %v, want %v",
					tc.raw,
					got,
					tc.want,
				)
			}
		})
	}
}
