package hiragana

import "math/rand/v2"

func BuildOptions(correct string, distractors []string) []string {
	options := make([]string, 0, len(distractors)+1)
	options = append(options, correct)
	options = append(options, distractors...)

	rand.Shuffle(len(options), func(i, j int) {
		options[i], options[j] = options[j], options[i]
	})

	return options
}
