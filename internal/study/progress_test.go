package study

import "testing"

func TestProgress_AccuracyPercent(t *testing.T) {
	tests := []struct {
		name     string
		progress Progress
		want     float64
	}{
		{
			name:     "no attempts",
			progress: Progress{},
			want:     0,
		},
		{
			name: "all correct",
			progress: Progress{
				TotalAttempts:   10,
				CorrectAttempts: 10,
			},
			want: 100,
		},
		{
			name: "some correct",
			progress: Progress{
				TotalAttempts:   4,
				CorrectAttempts: 3,
			},
			want: 75,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.progress.AccuracyPercent()
			if got != tc.want {
				t.Errorf("AccuracyPercent() = %v, want %v", got, tc.want)
			}
		})
	}
}
