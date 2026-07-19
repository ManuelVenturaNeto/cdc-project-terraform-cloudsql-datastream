// Command seed does the one-shot initial load (module 2).
package main

import (
	"context"
	"flag"
	"log"
	"math/rand/v2"

	"data-generator/internal/db"
	"data-generator/internal/gen"
)

func main() {
	nUsers := flag.Int("users", 20, "number of users to insert")
	nMovies := flag.Int("movies", 30, "number of movies to insert")
	nRentals := flag.Int("rentals", 50, "number of rentals to insert")
	flag.Parse()

	ctx := context.Background()
	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	userIDs := make([]int64, 0, *nUsers)
	for i := 0; i < *nUsers; i++ {
		name := gen.Name()
		var id int64
		err := pool.QueryRow(ctx,
			`INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id`,
			name, gen.Email(name)).Scan(&id)
		if err != nil {
			log.Printf("insert user: %v (skipping)", err)
			continue
		}
		userIDs = append(userIDs, id)
	}
	log.Printf("seeded %d users", len(userIDs))

	movieIDs := make([]int64, 0, *nMovies)
	for i := 0; i < *nMovies; i++ {
		var id int64
		err := pool.QueryRow(ctx,
			`INSERT INTO movies (title, genre, release_year, duration_minutes, synopsis)
			 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			gen.MovieTitle(), gen.Genre(), gen.ReleaseYear(), gen.DurationMinutes(), gen.Synopsis()).Scan(&id)
		if err != nil {
			log.Printf("insert movie: %v (skipping)", err)
			continue
		}
		movieIDs = append(movieIDs, id)
	}
	log.Printf("seeded %d movies", len(movieIDs))

	if len(userIDs) == 0 || len(movieIDs) == 0 {
		log.Fatal("no users or movies were inserted; cannot seed rentals")
	}

	rentals := 0
	for i := 0; i < *nRentals; i++ {
		start, end := gen.RentalPeriod()
		userID := userIDs[rand.IntN(len(userIDs))]
		movieID := movieIDs[rand.IntN(len(movieIDs))]
		_, err := pool.Exec(ctx,
			`INSERT INTO rentals (user_id, movie_id, start_date, end_date) VALUES ($1, $2, $3, $4)`,
			userID, movieID, start, end)
		if err != nil {
			log.Printf("insert rental: %v (skipping)", err)
			continue
		}
		rentals++
	}
	log.Printf("seeded %d rentals", rentals)
	log.Println("seed complete")
}
