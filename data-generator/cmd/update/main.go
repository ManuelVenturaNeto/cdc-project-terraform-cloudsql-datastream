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
	"users", "addresses", "user_addresses", "orders", "order_location",
}

var weights = map[string]int{
	"users":          2,
	"addresses":      2,
	"user_addresses": 1,
	"orders":         4,
	"order_location": 5,
}

const tableRefreshInterval = 10 * time.Second

const updateUserAddressSQL = `UPDATE user_addresses
SET label = $1, is_default = NOT is_default
WHERE (user_id, address_id) = (
	SELECT user_id, address_id FROM user_addresses ORDER BY random() LIMIT 1
)
RETURNING user_id, address_id, is_default`

const updateOrderLocationSQL = `UPDATE order_location
SET latitude = round(latitude + $1, 6), longitude = round(longitude + $2, 6), updated_at = now()
WHERE id = (SELECT id FROM order_location ORDER BY random() LIMIT 1)
RETURNING id, order_id, latitude, longitude`

var updateOrderSQL = fmt.Sprintf(`UPDATE orders
SET status = %s, updated_at = now()
WHERE id = (SELECT id FROM orders WHERE status <> $1 ORDER BY random() LIMIT 1)
RETURNING id, status`, gen.StatusAdvanceCase("status"))

func main() {
	interval := flag.Duration("interval", time.Second, "time between updates")
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

	log.Printf("updating %q every %s (Ctrl+C to stop)", *table, *interval)
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	refresh := time.NewTicker(tableRefreshInterval)
	defer refresh.Stop()

	total := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("stopped after %d updates", total)
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
			if updateOne(ctx, pool, target) {
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

func updateOne(ctx context.Context, pool *pgxpool.Pool, table string) bool {
	var (
		err      error
		describe func() string
	)

	switch table {
	case "users":
		var id int64
		name := gen.Name()
		err = pool.QueryRow(ctx,
			`UPDATE users SET name = $1, phone = $2
			 WHERE id = (SELECT id FROM users ORDER BY random() LIMIT 1)
			 RETURNING id`,
			name, gen.Phone()).Scan(&id)
		describe = func() string { return fmt.Sprintf("id=%d name=%q", id, name) }
	case "addresses":
		var id int64
		complement, district := gen.Complement(), gen.District()
		err = pool.QueryRow(ctx,
			`UPDATE addresses SET complement = $1, district = $2
			 WHERE id = (SELECT id FROM addresses ORDER BY random() LIMIT 1)
			 RETURNING id`,
			complement, district).Scan(&id)
		describe = func() string { return fmt.Sprintf("id=%d district=%q", id, district) }
	case "user_addresses":
		var userID, addressID int64
		var isDefault bool
		err = pool.QueryRow(ctx, updateUserAddressSQL, gen.AddressLabel()).Scan(&userID, &addressID, &isDefault)
		describe = func() string {
			return fmt.Sprintf("user_id=%d address_id=%d is_default=%t", userID, addressID, isDefault)
		}
	case "orders":
		var id int64
		var status string
		err = pool.QueryRow(ctx, updateOrderSQL, gen.FinalStatus()).Scan(&id, &status)
		describe = func() string { return fmt.Sprintf("id=%d status=%s", id, status) }
	case "order_location":
		var id, orderID int64
		var latitude, longitude float64
		stepLat, stepLon := gen.CoordinateStep()
		err = pool.QueryRow(ctx, updateOrderLocationSQL, stepLat, stepLon).Scan(&id, &orderID, &latitude, &longitude)
		describe = func() string {
			return fmt.Sprintf("id=%d order_id=%d at=%.6f,%.6f", id, orderID, latitude, longitude)
		}
	}

	if errors.Is(err, pgx.ErrNoRows) {
		log.Printf("skip %s: no row available to update", table)
		return false
	}
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		log.Printf("UPDATE %s failed: %v", table, err)
		return false
	}
	log.Printf("UPDATE %s %s", table, describe())
	return true
}
