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
	"users", "addresses", "user_addresses", "orders", "user_orders", "order_location",
}

var weights = map[string]int{
	"users":          2,
	"addresses":      2,
	"user_addresses": 2,
	"orders":         4,
	"user_orders":    1,
	"order_location": 3,
}

const tableRefreshInterval = 10 * time.Second

const insertUserAddressSQL = `INSERT INTO user_addresses (user_id, address_id, label, is_default)
SELECT u.id, a.id, $1, false
FROM (SELECT id FROM users ORDER BY random() LIMIT 1) u
CROSS JOIN (SELECT id FROM addresses ORDER BY random() LIMIT 1) a
WHERE NOT EXISTS (
	SELECT 1 FROM user_addresses ua WHERE ua.user_id = u.id AND ua.address_id = a.id
)
RETURNING user_id, address_id`

const insertOrderSQL = `WITH pick AS (
	SELECT user_id, address_id FROM user_addresses ORDER BY random() LIMIT 1
), new_order AS (
	INSERT INTO orders (status, total_amount, shipping_address_id)
	SELECT $1, $2, pick.address_id FROM pick
	RETURNING id
)
INSERT INTO user_orders (user_id, order_id)
SELECT pick.user_id, new_order.id FROM pick, new_order
RETURNING order_id`

const insertUserOrderSQL = `INSERT INTO user_orders (user_id, order_id)
SELECT u.id, o.id
FROM (SELECT id FROM users ORDER BY random() LIMIT 1) u
CROSS JOIN (SELECT id FROM orders ORDER BY random() LIMIT 1) o
WHERE NOT EXISTS (
	SELECT 1 FROM user_orders uo WHERE uo.user_id = u.id AND uo.order_id = o.id
)
RETURNING user_id, order_id`

const insertOrderLocationSQL = `INSERT INTO order_location (order_id, latitude, longitude)
SELECT o.id, $2, $3
FROM orders o
WHERE o.status = ANY($1)
  AND NOT EXISTS (SELECT 1 FROM order_location l WHERE l.order_id = o.id)
ORDER BY random()
LIMIT 1
RETURNING id, order_id`

func main() {
	interval := flag.Duration("interval", time.Second, "time between inserts")
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

	log.Printf("inserting into %q every %s (Ctrl+C to stop)", *table, *interval)
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	refresh := time.NewTicker(tableRefreshInterval)
	defer refresh.Stop()

	total := 0
	for {
		select {
		case <-ctx.Done():
			log.Printf("stopped after %d inserts", total)
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
			if insertOne(ctx, pool, target) {
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

func insertOne(ctx context.Context, pool *pgxpool.Pool, table string) bool {
	var (
		err      error
		describe func() string
	)

	switch table {
	case "users":
		var id int64
		name := gen.Name()
		err = pool.QueryRow(ctx,
			`INSERT INTO users (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
			name, gen.Email(name), gen.Phone()).Scan(&id)
		describe = func() string { return fmt.Sprintf("id=%d", id) }
	case "addresses":
		var addressID int64
		address := gen.NewAddress()
		err = pool.QueryRow(ctx,
			`INSERT INTO addresses (street, number, complement, district, city, state, zip_code, country)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
			address.Street, address.Number, address.Complement, address.District,
			address.City, address.State, address.ZipCode, address.Country).Scan(&addressID)
		describe = func() string { return fmt.Sprintf("id=%d city=%s", addressID, address.City) }
	case "user_addresses":
		var userID, addressID int64
		err = pool.QueryRow(ctx, insertUserAddressSQL, gen.AddressLabel()).Scan(&userID, &addressID)
		describe = func() string { return fmt.Sprintf("user_id=%d address_id=%d", userID, addressID) }
	case "orders":
		var orderID int64
		err = pool.QueryRow(ctx, insertOrderSQL, gen.InitialStatus(), gen.OrderAmount()).Scan(&orderID)
		describe = func() string { return fmt.Sprintf("id=%d", orderID) }
	case "user_orders":
		var userID, orderID int64
		err = pool.QueryRow(ctx, insertUserOrderSQL).Scan(&userID, &orderID)
		describe = func() string { return fmt.Sprintf("user_id=%d order_id=%d", userID, orderID) }
	case "order_location":
		var id, orderID int64
		latitude, longitude := gen.Coordinate()
		err = pool.QueryRow(ctx, insertOrderLocationSQL, gen.TrackableStatuses(), latitude, longitude).Scan(&id, &orderID)
		describe = func() string {
			return fmt.Sprintf("id=%d order_id=%d at=%.6f,%.6f", id, orderID, latitude, longitude)
		}
	}

	if errors.Is(err, pgx.ErrNoRows) {
		log.Printf("skip %s: no row matched the insert source", table)
		return false
	}
	if err != nil {
		if ctx.Err() != nil {
			return false
		}
		log.Printf("INSERT %s failed: %v", table, err)
		return false
	}
	log.Printf("INSERT %s %s", table, describe())
	return true
}
