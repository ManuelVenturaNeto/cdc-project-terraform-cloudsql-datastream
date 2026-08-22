package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	"data-generator/internal/db"
	"data-generator/internal/migrate"
	"data-generator/migrations"
)

func main() {
	status := flag.Bool("status", false, "list every migration and whether it is applied")
	highestVersion := flag.Int("to", 0, "highest version to apply; 0 applies everything pending")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	available, err := migrate.Load(migrations.FS)
	if err != nil {
		log.Fatalf("load migrations: %v", err)
	}
	if len(available) == 0 {
		log.Fatal("no migrations found")
	}

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	if *status {
		applied, err := migrate.Applied(ctx, pool)
		if err != nil {
			log.Fatalf("status: %v", err)
		}
		for _, migration := range available {
			if record, found := applied[migration.Version]; found {
				log.Printf("%-28s applied at %s", migration, record.AppliedAt.Format(time.RFC3339))
				continue
			}
			log.Printf("%-28s pending", migration)
		}
		return
	}

	done, applyErr := migrate.Apply(ctx, pool, available, *highestVersion)
	for _, migration := range done {
		log.Printf("applied %s", migration)
	}
	if applyErr != nil {
		log.Fatalf("migrate: %v", applyErr)
	}
	if len(done) == 0 {
		log.Println("nothing to apply")
	}
}
