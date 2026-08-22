package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

const presentTablesQuery = `SELECT table_name FROM information_schema.tables
	WHERE table_schema = 'public' AND table_name = ANY($1)`

func PresentTables(ctx context.Context, pool *pgxpool.Pool, candidates []string) ([]string, error) {
	rows, err := pool.Query(ctx, presentTablesQuery, candidates)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	existing := make(map[string]bool, len(candidates))
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		existing[tableName] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	present := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if existing[candidate] {
			present = append(present, candidate)
		}
	}
	return present, nil
}
