// Command setup creates the database schema (module 1).
// Idempotent: safe to run more than once.
package main

import (
	"context"
	"log"

	"data-generator/internal/db"
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
	log.Println("schema setup complete")
}
