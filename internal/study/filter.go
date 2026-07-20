package study

import "strings"

func ParseCharacters(raw string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	characters := make([]string, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			characters = append(characters, part)
		}
	}

	if len(characters) == 0 {
		return nil
	}

	return characters
}
