package migrate

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const historyDDL = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version INT PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

type Migration struct {
	Version int
	Name    string
	SQL     string
}

func (migration Migration) String() string {
	return fmt.Sprintf("%04d_%s", migration.Version, migration.Name)
}

type Record struct {
	Version   int
	Name      string
	AppliedAt time.Time
}

func Load(fsys fs.FS) ([]Migration, error) {
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, err
	}
	seen := make(map[int]string, len(files))
	loaded := make([]Migration, 0, len(files))
	for _, file := range files {
		base := strings.TrimSuffix(path.Base(file), ".sql")
		prefix, name, separated := strings.Cut(base, "_")
		if !separated || name == "" {
			return nil, fmt.Errorf("migration %q must be named <version>_<name>.sql", file)
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migration %q: %q is not a version number", file, prefix)
		}
		if previous, duplicated := seen[version]; duplicated {
			return nil, fmt.Errorf("migrations %q and %q share version %d", previous, file, version)
		}
		body, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, err
		}
		seen[version] = file
		loaded = append(loaded, Migration{Version: version, Name: name, SQL: string(body)})
	}
	sort.Slice(loaded, func(first, second int) bool { return loaded[first].Version < loaded[second].Version })
	return loaded, nil
}

func Applied(ctx context.Context, pool *pgxpool.Pool) (map[int]Record, error) {
	if _, err := pool.Exec(ctx, historyDDL); err != nil {
		return nil, fmt.Errorf("create schema_migrations: %w", err)
	}
	rows, err := pool.Query(ctx, `SELECT version, name, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := map[int]Record{}
	for rows.Next() {
		var record Record
		if err := rows.Scan(&record.Version, &record.Name, &record.AppliedAt); err != nil {
			return nil, err
		}
		applied[record.Version] = record
	}
	return applied, rows.Err()
}

func Apply(ctx context.Context, pool *pgxpool.Pool, migrations []Migration, highestVersion int) ([]Migration, error) {
	applied, err := Applied(ctx, pool)
	if err != nil {
		return nil, err
	}
	done := make([]Migration, 0, len(migrations))
	for _, migration := range migrations {
		if highestVersion > 0 && migration.Version > highestVersion {
			break
		}
		if _, alreadyApplied := applied[migration.Version]; alreadyApplied {
			continue
		}
		if err := applyOne(ctx, pool, migration); err != nil {
			return done, fmt.Errorf("apply %s: %w", migration, err)
		}
		done = append(done, migration)
	}
	return done, nil
}

func applyOne(ctx context.Context, pool *pgxpool.Pool, migration Migration) error {
	transaction, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer transaction.Rollback(ctx)

	if _, err := transaction.Exec(ctx, migration.SQL); err != nil {
		return err
	}
	if _, err := transaction.Exec(ctx,
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		migration.Version, migration.Name); err != nil {
		return err
	}
	return transaction.Commit(ctx)
}
