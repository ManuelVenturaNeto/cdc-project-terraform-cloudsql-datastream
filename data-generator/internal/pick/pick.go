package pick

import "math/rand/v2"

func Weighted(options []string, weights map[string]int) string {
	total := 0
	for _, option := range options {
		if weight := weights[option]; weight > 0 {
			total += weight
		}
	}
	if total == 0 {
		return ""
	}

	draw := rand.IntN(total)
	for _, option := range options {
		weight := weights[option]
		if weight <= 0 {
			continue
		}
		if draw < weight {
			return option
		}
		draw -= weight
	}
	return ""
}
