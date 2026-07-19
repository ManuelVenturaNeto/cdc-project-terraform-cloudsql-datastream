// Command update keeps changing random existing rows on an interval (module 4).
// Runs until Ctrl+C. Each tick updates one random row in one table.
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
	interval := flag.Duration("interval", time.Second, "time between updates")
	table := flag.String("table", "all", "target table: users, movies, rentals or all")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	log.Printf("updating %q every %s (Ctrl+C to stop)", *table, *interval)
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	total := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("stopped after %d updates", total)
			return
		case <-ticker.C:
			target := *table
			if target == "all" {
				target = []string{"users", "movies", "rentals"}[rand.IntN(3)]
			}
			if updateOne(ctx, pool, target) {
				total++
			}
		}
	}
}

func updateOne(ctx context.Context, pool *pgxpool.Pool, table string) bool {
	var id int64
	var err error

	switch table {
	case "users":
		err = pool.QueryRow(ctx,
			`UPDATE users SET name = $1
			 WHERE id = (SELECT id FROM users ORDER BY random() LIMIT 1)
			 RETURNING id`,
			gen.Name()).Scan(&id)
	case "movies":
		err = pool.QueryRow(ctx,
			`UPDATE movies SET synopsis = $1, duration_minutes = $2
			 WHERE id = (SELECT id FROM movies ORDER BY random() LIMIT 1)
			 RETURNING id`,
			gen.Synopsis(), gen.DurationMinutes()).Scan(&id)
	case "rentals":
		err = pool.QueryRow(ctx,
			`UPDATE rentals SET end_date = end_date + 1
			 WHERE id = (SELECT id FROM rentals ORDER BY random() LIMIT 1)
			 RETURNING id`).Scan(&id)
	default:
		log.Fatalf("unknown table %q", table)
	}

	if errors.Is(err, pgx.ErrNoRows) {
		log.Printf("skip %s: table is empty", table)
		return false
	}
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		log.Printf("UPDATE %s failed: %v", table, err)
		return false
	}
	log.Printf("UPDATE %s id=%d", table, id)
	return true
}
