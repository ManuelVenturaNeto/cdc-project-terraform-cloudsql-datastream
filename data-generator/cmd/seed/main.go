package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"math/rand/v2"
	"slices"

	"data-generator/internal/db"
	"data-generator/internal/gen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const insertOrderSQL = `WITH new_order AS (
	INSERT INTO orders (status, total_amount, shipping_address_id, placed_at, updated_at)
	VALUES ($1, $2, $3, $4, $4)
	RETURNING id
)
INSERT INTO user_orders (user_id, order_id)
SELECT $5, new_order.id FROM new_order
RETURNING order_id`

func main() {
	userCount := flag.Int("users", 20, "number of users to insert")
	addressCount := flag.Int("addresses", 30, "number of addresses to insert")
	orderCount := flag.Int("orders", 50, "number of orders to insert")
	flag.Parse()

	ctx := context.Background()
	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	userIDs := seedUsers(ctx, pool, *userCount)
	log.Printf("seeded %d users", len(userIDs))

	addressIDs := seedAddresses(ctx, pool, *addressCount)
	log.Printf("seeded %d addresses", len(addressIDs))

	if len(userIDs) == 0 || len(addressIDs) == 0 {
		log.Fatal("no users or addresses were inserted; cannot seed orders")
	}

	links := seedUserAddresses(ctx, pool, userIDs, addressIDs)
	log.Printf("seeded %d user_addresses", countLinks(links))

	tracked := seedOrders(ctx, pool, links, *orderCount)
	log.Printf("seeded %d orders", len(tracked.orderIDs))

	present, err := db.PresentTables(ctx, pool, []string{"order_location"})
	if err != nil {
		log.Fatalf("list tables: %v", err)
	}
	if !slices.Contains(present, "order_location") {
		log.Println("order_location not migrated yet; skipping location seed")
		log.Println("seed complete")
		return
	}
	log.Printf("seeded %d order_location rows", seedLocations(ctx, pool, tracked.trackableIDs))
	log.Println("seed complete")
}

func seedUsers(ctx context.Context, pool *pgxpool.Pool, count int) []int64 {
	userIDs := make([]int64, 0, count)
	for range count {
		name := gen.Name()
		var userID int64
		err := pool.QueryRow(ctx,
			`INSERT INTO users (name, email, phone) VALUES ($1, $2, $3) RETURNING id`,
			name, gen.Email(name), gen.Phone()).Scan(&userID)
		if err != nil {
			log.Printf("insert user: %v (skipping)", err)
			continue
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs
}

func seedAddresses(ctx context.Context, pool *pgxpool.Pool, count int) []int64 {
	addressIDs := make([]int64, 0, count)
	for range count {
		address := gen.NewAddress()
		var addressID int64
		err := pool.QueryRow(ctx,
			`INSERT INTO addresses (street, number, complement, district, city, state, zip_code, country)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
			address.Street, address.Number, address.Complement, address.District,
			address.City, address.State, address.ZipCode, address.Country).Scan(&addressID)
		if err != nil {
			log.Printf("insert address: %v (skipping)", err)
			continue
		}
		addressIDs = append(addressIDs, addressID)
	}
	return addressIDs
}

func seedUserAddresses(ctx context.Context, pool *pgxpool.Pool, userIDs, addressIDs []int64) map[int64][]int64 {
	links := make(map[int64][]int64, len(userIDs))
	for _, userID := range userIDs {
		wanted := 1 + rand.IntN(2)
		for range wanted {
			addressID := addressIDs[rand.IntN(len(addressIDs))]
			if contains(links[userID], addressID) {
				continue
			}
			_, err := pool.Exec(ctx,
				`INSERT INTO user_addresses (user_id, address_id, label, is_default) VALUES ($1, $2, $3, $4)`,
				userID, addressID, gen.AddressLabel(), len(links[userID]) == 0)
			if err != nil {
				log.Printf("insert user_address: %v (skipping)", err)
				continue
			}
			links[userID] = append(links[userID], addressID)
		}
	}
	return links
}

type seededOrders struct {
	orderIDs     []int64
	trackableIDs []int64
}

func seedOrders(ctx context.Context, pool *pgxpool.Pool, links map[int64][]int64, count int) seededOrders {
	owners := make([]int64, 0, len(links))
	for userID, addresses := range links {
		if len(addresses) > 0 {
			owners = append(owners, userID)
		}
	}
	if len(owners) == 0 {
		log.Fatal("no user has an address; cannot seed orders")
	}

	var seeded seededOrders
	for range count {
		userID := owners[rand.IntN(len(owners))]
		addresses := links[userID]
		addressID := addresses[rand.IntN(len(addresses))]
		status := gen.RandomStatus()

		var orderID int64
		err := pool.QueryRow(ctx, insertOrderSQL,
			status, gen.OrderAmount(), addressID, gen.PlacedAt(), userID).Scan(&orderID)
		if err != nil {
			log.Printf("insert order: %v (skipping)", err)
			continue
		}
		seeded.orderIDs = append(seeded.orderIDs, orderID)
		if gen.IsTrackable(status) {
			seeded.trackableIDs = append(seeded.trackableIDs, orderID)
		}
	}
	return seeded
}

func seedLocations(ctx context.Context, pool *pgxpool.Pool, orderIDs []int64) int {
	seeded := 0
	for _, orderID := range orderIDs {
		latitude, longitude := gen.Coordinate()
		_, err := pool.Exec(ctx,
			`INSERT INTO order_location (order_id, latitude, longitude) VALUES ($1, $2, $3)
			 ON CONFLICT (order_id) DO NOTHING`,
			orderID, latitude, longitude)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("insert order_location: %v (skipping)", err)
			continue
		}
		seeded++
	}
	return seeded
}

func contains(list []int64, wanted int64) bool {
	for _, value := range list {
		if value == wanted {
			return true
		}
	}
	return false
}

func countLinks(links map[int64][]int64) int {
	total := 0
	for _, addresses := range links {
		total += len(addresses)
	}
	return total
}
