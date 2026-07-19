// Command setup creates the database schema and the Datastream prerequisites (module 1).
// Idempotent: safe to run more than once.
// Must run as a superuser (postgres) and before `terraform apply -var enable_stream=true`.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"data-generator/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

var tables = []struct {
	name string
	ddl  string
}{
	{"movies", `
		CREATE TABLE IF NOT EXISTS movies (
			id SERIAL PRIMARY KEY,
			title TEXT NOT NULL,
			genre TEXT,
			release_year INT,
			duration_minutes INT,
			synopsis TEXT
		)`},
	{"users", `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL UNIQUE
		)`},
	{"rentals", `
		CREATE TABLE IF NOT EXISTS rentals (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL REFERENCES users(id),
			movie_id INT NOT NULL REFERENCES movies(id),
			start_date DATE NOT NULL,
			end_date DATE NOT NULL
		)`},
}

func main() {
	var (
		dsUser      = flag.String("datastream-user", "datastream", "role Datastream logs in as")
		publication = flag.String("publication", "ds_publication", "publication name, must match Terraform")
		slot        = flag.String("replication-slot", "ds_replication_slot", "slot name, must match Terraform")
	)
	flag.Parse()

	ctx := context.Background()
	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	for _, t := range tables {
		if _, err := pool.Exec(ctx, t.ddl); err != nil {
			log.Fatalf("create table %s: %v", t.name, err)
		}
		log.Printf("table %s ready", t.name)
	}

	if err := setupCDC(ctx, pool, *dsUser, *publication, *slot); err != nil {
		log.Fatalf("cdc setup: %v", err)
	}
	log.Println("schema and cdc prerequisites complete")
}

// setupCDC grants the Datastream role what it needs and creates the publication and slot.
// Terraform cannot express any of this: google_sql_user only manages name and password.
func setupCDC(ctx context.Context, pool *pgxpool.Pool, dsUser, publication, slot string) error {
	// Identifiers cannot be bound as parameters, so they are quoted instead
	grants := []string{
		fmt.Sprintf(`ALTER USER %s WITH REPLICATION`, quoteIdent(dsUser)),
		fmt.Sprintf(`GRANT USAGE ON SCHEMA public TO %s`, quoteIdent(dsUser)),
		fmt.Sprintf(`GRANT SELECT ON ALL TABLES IN SCHEMA public TO %s`, quoteIdent(dsUser)),
		// Covers tables created after this run, by seed and by later migrations
		fmt.Sprintf(`ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO %s`, quoteIdent(dsUser)),
	}
	for _, stmt := range grants {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	log.Printf("role %s ready for replication", dsUser)

	var hasPublication bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = $1)`, publication,
	).Scan(&hasPublication); err != nil {
		return fmt.Errorf("check publication: %w", err)
	}
	if hasPublication {
		log.Printf("publication %s already present", publication)
	} else {
		// FOR ALL TABLES also picks up tables created later
		if _, err := pool.Exec(ctx,
			fmt.Sprintf(`CREATE PUBLICATION %s FOR ALL TABLES`, quoteIdent(publication)),
		); err != nil {
			return fmt.Errorf("create publication: %w", err)
		}
		log.Printf("publication %s created", publication)
	}

	// On Cloud SQL not even postgres has REPLICATION, and the slot below needs it
	var self string
	if err := pool.QueryRow(ctx, `SELECT current_user`).Scan(&self); err != nil {
		return fmt.Errorf("current_user: %w", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`ALTER USER %s WITH REPLICATION`, quoteIdent(self))); err != nil {
		return fmt.Errorf("grant replication to %s: %w", self, err)
	}

	var hasSlot bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name = $1)`, slot,
	).Scan(&hasSlot); err != nil {
		return fmt.Errorf("check replication slot: %w", err)
	}
	if hasSlot {
		log.Printf("replication slot %s already present", slot)
		return nil
	}
	if _, err := pool.Exec(ctx,
		`SELECT pg_create_logical_replication_slot($1, 'pgoutput')`, slot,
	); err != nil {
		return fmt.Errorf("create replication slot: %w", err)
	}
	log.Printf("replication slot %s created", slot)
	return nil
}

// quoteIdent escapes a SQL identifier for interpolation into DDL.
func quoteIdent(s string) string {
	out := make([]rune, 0, len(s)+2)
	out = append(out, '"')
	for _, r := range s {
		if r == '"' {
			out = append(out, '"')
		}
		out = append(out, r)
	}
	return string(append(out, '"'))
}
