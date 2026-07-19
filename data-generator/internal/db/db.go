package db

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// Connect loads DB_* env vars (from .env when present) and opens a pgx
// connection pool, verifying it with a ping before returning.
func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	_ = godotenv.Load() // .env is optional: vars may come from the real environment

	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	name := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	if host == "" || user == "" || name == "" {
		return nil, fmt.Errorf("DB_HOST, DB_USER and DB_NAME must be set")
	}
	if port == "" {
		port = "5432"
	}
	if user == "" {
		user = "postgres"
	}
	if name == "" {
		name = "main_movie"
	}
	if sslmode == "" {
		sslmode = "require"
	}

	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     host + ":" + port,
		Path:     "/" + name,
		RawQuery: "sslmode=" + sslmode,
	}

	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}
