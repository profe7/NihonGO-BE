package hiragana

type Card struct {
	ID        int64  `json:"id"        db:"id"`
	Character string `json:"character" db:"character"`
	Romaji    string `json:"romaji"    db:"romaji"`
}
