package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"data-generator/internal/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	var (
		datastreamUser = flag.String("datastream-user", "datastream", "role Datastream logs in as")
		publication    = flag.String("publication", "ds_publication", "publication name, must match Terraform")
		slot           = flag.String("replication-slot", "ds_replication_slot", "slot name, must match Terraform")
	)
	flag.Parse()

	ctx := context.Background()
	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	if err := setupCDC(ctx, pool, *datastreamUser, *publication, *slot); err != nil {
		log.Fatalf("cdc setup: %v", err)
	}
	log.Println("cdc prerequisites complete")
}

func setupCDC(ctx context.Context, pool *pgxpool.Pool, datastreamUser, publication, slot string) error {
	grants := []string{
		fmt.Sprintf(`ALTER USER %s WITH REPLICATION`, quoteIdentifier(datastreamUser)),
		fmt.Sprintf(`GRANT USAGE ON SCHEMA public TO %s`, quoteIdentifier(datastreamUser)),
		fmt.Sprintf(`GRANT SELECT ON ALL TABLES IN SCHEMA public TO %s`, quoteIdentifier(datastreamUser)),
		fmt.Sprintf(`ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO %s`,
			quoteIdentifier(datastreamUser)),
	}
	for _, statement := range grants {
		if _, err := pool.Exec(ctx, statement); err != nil {
			return fmt.Errorf("%s: %w", statement, err)
		}
	}
	log.Printf("role %s ready for replication", datastreamUser)

	var hasPublication bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = $1)`, publication,
	).Scan(&hasPublication); err != nil {
		return fmt.Errorf("check publication: %w", err)
	}
	if hasPublication {
		log.Printf("publication %s already present", publication)
	} else {
		if _, err := pool.Exec(ctx,
			fmt.Sprintf(`CREATE PUBLICATION %s FOR ALL TABLES`, quoteIdentifier(publication)),
		); err != nil {
			return fmt.Errorf("create publication: %w", err)
		}
		log.Printf("publication %s created", publication)
	}

	var currentUser string
	if err := pool.QueryRow(ctx, `SELECT current_user`).Scan(&currentUser); err != nil {
		return fmt.Errorf("current_user: %w", err)
	}
	if _, err := pool.Exec(ctx,
		fmt.Sprintf(`ALTER USER %s WITH REPLICATION`, quoteIdentifier(currentUser))); err != nil {
		return fmt.Errorf("grant replication to %s: %w", currentUser, err)
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

func quoteIdentifier(identifier string) string {
	quoted := make([]rune, 0, len(identifier)+2)
	quoted = append(quoted, '"')
	for _, character := range identifier {
		if character == '"' {
			quoted = append(quoted, '"')
		}
		quoted = append(quoted, character)
	}
	return string(append(quoted, '"'))
}
