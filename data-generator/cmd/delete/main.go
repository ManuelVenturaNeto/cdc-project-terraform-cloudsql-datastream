package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"data-generator/internal/db"
	"data-generator/internal/gen"
	"data-generator/internal/pick"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var candidates = []string{
	"order_location", "user_orders", "orders", "addresses", "users",
}

var weights = map[string]int{
	"order_location": 4,
	"user_orders":    1,
	"orders":         3,
	"addresses":      2,
	"users":          1,
}

const tableRefreshInterval = 10 * time.Second

const deleteOrderLocationSQL = `DELETE FROM order_location
WHERE id = (
	SELECT l.id FROM order_location l
	JOIN orders o ON o.id = l.order_id
	WHERE o.status = $1
	ORDER BY random() LIMIT 1
)
RETURNING id, order_id`

const deleteUserOrderSQL = `DELETE FROM user_orders
WHERE (user_id, order_id) = (
	SELECT uo.user_id, uo.order_id FROM user_orders uo
	WHERE (SELECT count(*) FROM user_orders peer WHERE peer.order_id = uo.order_id) > 1
	ORDER BY random() LIMIT 1
)
RETURNING user_id, order_id`

const pickDeliveredOrderSQL = `SELECT id FROM orders WHERE status = $1 ORDER BY random() LIMIT 1`

const pickUnusedAddressSQL = `SELECT a.id FROM addresses a
WHERE NOT EXISTS (SELECT 1 FROM orders o WHERE o.shipping_address_id = a.id)
ORDER BY random() LIMIT 1`

const pickOrderlessUserSQL = `SELECT u.id FROM users u
WHERE NOT EXISTS (SELECT 1 FROM user_orders uo WHERE uo.user_id = u.id)
ORDER BY random() LIMIT 1`

func main() {
	interval := flag.Duration("interval", time.Second, "time between deletes")
	table := flag.String("table", "all", "target table: "+strings.Join(candidates, ", ")+" or all")
	flag.Parse()

	if *table != "all" && !slices.Contains(candidates, *table) {
		log.Fatalf("unknown table %q", *table)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	present, err := db.PresentTables(ctx, pool, candidates)
	if err != nil {
		log.Fatalf("list tables: %v", err)
	}

	log.Printf("deleting from %q every %s (Ctrl+C to stop)", *table, *interval)
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	refresh := time.NewTicker(tableRefreshInterval)
	defer refresh.Stop()

	total := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("stopped after %d deletes", total)
			return

		case <-refresh.C:
			refreshed, err := db.PresentTables(ctx, pool, candidates)
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("list tables: %v", err)
				}
				continue
			}
			present = refreshed

		case <-ticker.C:
			target := chooseTarget(present, *table)
			if target == "" {
				continue
			}
			if deleteOne(ctx, pool, present, target) {
				total++
			}
		}
	}
}

func chooseTarget(present []string, requested string) string {
	if requested == "all" {
		target := pick.Weighted(present, weights)
		if target == "" {
			log.Println("skip: no target table exists yet, run ./cmd/migrate")
		}
		return target
	}
	if !slices.Contains(present, requested) {
		log.Printf("skip %s: table does not exist yet", requested)
		return ""
	}
	return requested
}

func deleteOne(ctx context.Context, pool *pgxpool.Pool, present []string, table string) bool {
	var (
		err      error
		describe func() string
	)

	switch table {
	case "order_location":
		var id, orderID int64
		err = pool.QueryRow(ctx, deleteOrderLocationSQL, gen.FinalStatus()).Scan(&id, &orderID)
		describe = func() string { return fmt.Sprintf("id=%d order_id=%d", id, orderID) }
	case "user_orders":
		var userID, orderID int64
		err = pool.QueryRow(ctx, deleteUserOrderSQL).Scan(&userID, &orderID)
		describe = func() string { return fmt.Sprintf("user_id=%d order_id=%d", userID, orderID) }
	case "orders":
		var id int64
		id, err = deleteOrder(ctx, pool, present)
		describe = func() string { return fmt.Sprintf("id=%d", id) }
	case "addresses":
		var id int64
		id, err = deleteCascading(ctx, pool, pickUnusedAddressSQL, nil,
			`DELETE FROM user_addresses WHERE address_id = $1`,
			`DELETE FROM addresses WHERE id = $1`)
		describe = func() string { return fmt.Sprintf("id=%d", id) }
	case "users":
		var id int64
		id, err = deleteCascading(ctx, pool, pickOrderlessUserSQL, nil,
			`DELETE FROM user_addresses WHERE user_id = $1`,
			`DELETE FROM users WHERE id = $1`)
		describe = func() string { return fmt.Sprintf("id=%d", id) }
	}

	if errors.Is(err, pgx.ErrNoRows) {
		log.Printf("skip %s: no deletable row, every candidate is still referenced", table)
		return false
	}
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		log.Printf("DELETE %s failed: %v", table, err)
		return false
	}
	log.Printf("DELETE %s %s", table, describe())
	return true
}

func deleteOrder(ctx context.Context, pool *pgxpool.Pool, present []string) (int64, error) {
	statements := []string{`DELETE FROM user_orders WHERE order_id = $1`}
	if slices.Contains(present, "order_location") {
		statements = append([]string{`DELETE FROM order_location WHERE order_id = $1`}, statements...)
	}
	statements = append(statements, `DELETE FROM orders WHERE id = $1`)
	return deleteCascading(ctx, pool, pickDeliveredOrderSQL, []any{gen.FinalStatus()}, statements...)
}

func deleteCascading(ctx context.Context, pool *pgxpool.Pool, pickSQL string, pickArgs []any, statements ...string) (int64, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var id int64
	if err := tx.QueryRow(ctx, pickSQL, pickArgs...).Scan(&id); err != nil {
		return 0, err
	}
	for _, stmt := range statements {
		if _, err := tx.Exec(ctx, stmt, id); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit(ctx)
}
