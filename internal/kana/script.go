package kana

import "fmt"

type Script string

const (
	Hiragana Script = "hiragana"
	Katakana Script = "katakana"
)

type scriptConfig struct {
	cardsTable    string
	attemptsTable string
}

func (s Script) config() scriptConfig {
	switch s {
	case Hiragana:
		return scriptConfig{
			cardsTable:    "hiragana",
			attemptsTable: "hiragana_attempts",
		}
	case Katakana:
		return scriptConfig{
			cardsTable:    "katakana",
			attemptsTable: "katakana_attempts",
		}
	default:
		panic(fmt.Sprintf("unsupported kana script %q", s))
	}
}
