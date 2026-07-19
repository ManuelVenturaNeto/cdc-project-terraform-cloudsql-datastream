// Command insert keeps inserting random rows on an interval (module 3).
// Runs until Ctrl+C. Each tick inserts into one table: users, movies or
// rentals, chosen at random (or fixed via -table).
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"math/rand/v2"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"data-generator/internal/db"
	"data-generator/internal/gen"
)

func main() {
	interval := flag.Duration("interval", time.Second, "time between inserts")
	table := flag.String("table", "all", "target table: users, movies, rentals or all")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	log.Printf("inserting into %q every %s (Ctrl+C to stop)", *table, *interval)
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	total := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("stopped after %d inserts", total)
			return
		case <-ticker.C:
			target := *table
			if target == "all" {
				target = []string{"users", "movies", "rentals"}[rand.IntN(3)]
			}
			if insertOne(ctx, pool, target) {
				total++
			}
		}
	}
}

func insertOne(ctx context.Context, pool *pgxpool.Pool, table string) bool {
	var id int64
	var err error

	switch table {
	case "users":
		name := gen.Name()
		err = pool.QueryRow(ctx,
			`INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id`,
			name, gen.Email(name)).Scan(&id)
	case "movies":
		err = pool.QueryRow(ctx,
			`INSERT INTO movies (title, genre, release_year, duration_minutes, synopsis)
			 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			gen.MovieTitle(), gen.Genre(), gen.ReleaseYear(), gen.DurationMinutes(), gen.Synopsis()).Scan(&id)
	case "rentals":
		start, end := gen.RentalPeriod()
		// Random existing user/movie; ORDER BY random() is fine at this scale.
		err = pool.QueryRow(ctx,
			`INSERT INTO rentals (user_id, movie_id, start_date, end_date)
			 SELECT u.id, m.id, $1, $2
			 FROM (SELECT id FROM users ORDER BY random() LIMIT 1) u,
			      (SELECT id FROM movies ORDER BY random() LIMIT 1) m
			 RETURNING id`,
			start, end).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			log.Println("skip rentals: need at least one user and one movie")
			return false
		}
	default:
		log.Fatalf("unknown table %q", table)
	}

	if err != nil {
		if ctx.Err() != nil {
			return false // shutting down; the error is just the canceled query
		}
		log.Printf("INSERT %s failed: %v", table, err)
		return false
	}
	log.Printf("INSERT %s id=%d", table, id)
	return true
}
