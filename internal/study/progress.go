package study

type Progress struct {
	TotalAttempts   int64
	CorrectAttempts int64
}

func (p Progress) AccuracyPercent() float64 {
	if p.TotalAttempts == 0 {
		return 0
	}
	return float64(p.CorrectAttempts) / float64(p.TotalAttempts) * 100
}
