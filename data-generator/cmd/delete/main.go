// Command delete keeps removing random rows on an interval (module 5).
// Runs until Ctrl+C. Rentals are deleted more often; users and movies are
// only deleted when no rental references them, so foreign keys never break.
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
)

func main() {
	interval := flag.Duration("interval", time.Second, "time between deletes")
	table := flag.String("table", "all", "target table: users, movies, rentals or all")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	log.Printf("deleting from %q every %s (Ctrl+C to stop)", *table, *interval)
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	total := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("stopped after %d deletes", total)
			return
		case <-ticker.C:
			target := *table
			if target == "all" {
				// Rentals outnumber the rest and have no dependents, so favor them.
				switch n := rand.IntN(10); {
				case n < 6:
					target = "rentals"
				case n < 8:
					target = "movies"
				default:
					target = "users"
				}
			}
			if deleteOne(ctx, pool, target) {
				total++
			}
		}
	}
}

func deleteOne(ctx context.Context, pool *pgxpool.Pool, table string) bool {
	var id int64
	var err error

	switch table {
	case "rentals":
		err = pool.QueryRow(ctx,
			`DELETE FROM rentals
			 WHERE id = (SELECT id FROM rentals ORDER BY random() LIMIT 1)
			 RETURNING id`).Scan(&id)
	case "movies":
		err = pool.QueryRow(ctx,
			`DELETE FROM movies
			 WHERE id = (SELECT m.id FROM movies m
			             WHERE NOT EXISTS (SELECT 1 FROM rentals r WHERE r.movie_id = m.id)
			             ORDER BY random() LIMIT 1)
			 RETURNING id`).Scan(&id)
	case "users":
		err = pool.QueryRow(ctx,
			`DELETE FROM users
			 WHERE id = (SELECT u.id FROM users u
			             WHERE NOT EXISTS (SELECT 1 FROM rentals r WHERE r.user_id = u.id)
			             ORDER BY random() LIMIT 1)
			 RETURNING id`).Scan(&id)
	default:
		log.Fatalf("unknown table %q", table)
	}

	if errors.Is(err, pgx.ErrNoRows) {
		log.Printf("skip %s: no deletable row (empty or all referenced by rentals)", table)
		return false
	}
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		log.Printf("DELETE %s failed: %v", table, err)
		return false
	}
	log.Printf("DELETE %s id=%d", table, id)
	return true
}
