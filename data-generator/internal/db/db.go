package db

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

const (
	defaultPort            = "5432"
	defaultSSLMode         = "require"
	defaultMaxConns        = 10
	defaultMinConns        = 2
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
	envFileSearchDepth     = 4
)

func Connect(ctx context.Context) (*pgxpool.Pool, error) {
	if err := loadEnvFile(); err != nil {
		return nil, fmt.Errorf("load env file: %w", err)
	}

	host := os.Getenv("DB_HOST")
	user := os.Getenv("DB_USER")
	name := os.Getenv("DB_NAME")
	if host == "" || user == "" || name == "" {
		return nil, errors.New("DB_HOST, DB_USER and DB_NAME must be set")
	}

	dsn := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, os.Getenv("DB_PASSWORD")),
		Host:     net.JoinHostPort(host, envString("DB_PORT", defaultPort)),
		Path:     "/" + name,
		RawQuery: url.Values{"sslmode": {envString("DB_SSLMODE", defaultSSLMode)}}.Encode(),
	}

	poolConfig, err := pgxpool.ParseConfig(dsn.String())
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	if err := applyPoolLimits(poolConfig); err != nil {
		return nil, err
	}
	poolConfig.ConnConfig.RuntimeParams["application_name"] = applicationName()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping %s: %w", dsn.Host, err)
	}
	return pool, nil
}

func applyPoolLimits(poolConfig *pgxpool.Config) error {
	maxConns, err := envInt("DB_MAX_CONNS", defaultMaxConns)
	if err != nil {
		return err
	}
	minConns, err := envInt("DB_MIN_CONNS", defaultMinConns)
	if err != nil {
		return err
	}
	if maxConns < 1 {
		return fmt.Errorf("DB_MAX_CONNS must be at least 1, got %d", maxConns)
	}
	if minConns < 0 || minConns > maxConns {
		return fmt.Errorf("DB_MIN_CONNS must be between 0 and DB_MAX_CONNS (%d), got %d", maxConns, minConns)
	}

	maxLifetime, err := envDuration("DB_CONN_MAX_LIFETIME", defaultConnMaxLifetime)
	if err != nil {
		return err
	}
	maxIdleTime, err := envDuration("DB_CONN_MAX_IDLE_TIME", defaultConnMaxIdleTime)
	if err != nil {
		return err
	}

	poolConfig.MaxConns = int32(maxConns)
	poolConfig.MinConns = int32(minConns)
	poolConfig.MaxConnLifetime = maxLifetime
	poolConfig.MaxConnIdleTime = maxIdleTime
	return nil
}

func loadEnvFile() error {
	if explicitPath := os.Getenv("DB_ENV_FILE"); explicitPath != "" {
		return godotenv.Load(explicitPath)
	}
	directory, err := os.Getwd()
	if err != nil {
		return nil
	}
	for level := 0; level < envFileSearchDepth; level++ {
		candidate := filepath.Join(directory, ".env")
		if _, err := os.Stat(candidate); err == nil {
			return godotenv.Load(candidate)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
		directory = parent
	}
	return nil
}

func applicationName() string {
	return "data-generator/" + filepath.Base(os.Args[0])
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not an integer", key, raw)
	}
	return parsed, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s=%q is not a duration", key, raw)
	}
	return parsed, nil
}
