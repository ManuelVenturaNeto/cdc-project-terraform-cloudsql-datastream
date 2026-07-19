// Package gen produces random but plausible data for the movies/users/rentals
// schema, so CDC events carry values that look real instead of gibberish.
package gen

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
)

var firstNames = []string{
	"Ana", "Bruno", "Carla", "Diego", "Elisa", "Felipe", "Gabriela", "Hugo",
	"Isabela", "Joao", "Karen", "Lucas", "Marina", "Nina", "Otavio", "Paula",
	"Rafael", "Sofia", "Thiago", "Vera",
}

var lastNames = []string{
	"Almeida", "Barbosa", "Cardoso", "Duarte", "Ferreira", "Gomes", "Lima",
	"Martins", "Nunes", "Oliveira", "Pereira", "Ribeiro", "Santos", "Silva", "Souza",
}

var genres = []string{
	"Action", "Comedy", "Drama", "Horror", "Sci-Fi", "Romance", "Thriller",
	"Documentary", "Animation",
}

var titleAdjectives = []string{
	"Lost", "Silent", "Golden", "Broken", "Hidden", "Final", "Eternal", "Dark",
	"Distant", "Burning",
}

var titleNouns = []string{
	"Empire", "River", "Memory", "Horizon", "Garden", "Signal", "Winter",
	"Voyage", "Shadow", "Promise",
}

func pick(list []string) string {
	return list[rand.IntN(len(list))]
}

func Name() string {
	return pick(firstNames) + " " + pick(lastNames)
}

// Email derives an address from a name plus a random suffix wide enough to
// make collisions with the UNIQUE constraint unlikely.
func Email(name string) string {
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "."))
	return fmt.Sprintf("%s.%d@example.com", slug, rand.IntN(1_000_000_000))
}

func MovieTitle() string {
	return fmt.Sprintf("The %s %s %d", pick(titleAdjectives), pick(titleNouns), rand.IntN(1000))
}

func Genre() string {
	return pick(genres)
}

func ReleaseYear() int {
	return 1960 + rand.IntN(67) // 1960..2026
}

func DurationMinutes() int {
	return 60 + rand.IntN(141) // 60..200
}

func Synopsis() string {
	return fmt.Sprintf("A %s story about a %s.",
		strings.ToLower(pick(titleAdjectives)), strings.ToLower(pick(titleNouns)))
}

// RentalPeriod returns a start date within the last 30 days and an end date
// 1 to 14 days after it.
func RentalPeriod() (start, end time.Time) {
	start = time.Now().AddDate(0, 0, -rand.IntN(30))
	end = start.AddDate(0, 0, 1+rand.IntN(14))
	return start, end
}
