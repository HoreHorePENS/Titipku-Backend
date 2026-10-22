package database

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationLockKey int64 = 7420152301

func Migrate(ctx context.Context, pool *pgxpool.Pool, files fs.FS, directory string) error {
	connection, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer connection.Release()

	defer func() {
		unlockCtx, cancelUnlock := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelUnlock()
		_, _ = connection.Exec(unlockCtx, "SELECT pg_advisory_unlock($1)", migrationLockKey)
	}()

	if _, err := connection.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}

	if _, err := connection.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	migrationNames, err := sqlMigrations(files, directory)
	if err != nil {
		return err
	}

	for _, name := range migrationNames {
		version := strings.TrimSuffix(name, path.Ext(name))

		var applied bool
		if err := connection.QueryRow(
			ctx,
			"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)",
			version,
		).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if applied {
			continue
		}

		content, err := fs.ReadFile(files, path.Join(directory, name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}

		transaction, err := connection.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", version, err)
		}

		if _, err := transaction.Conn().PgConn().Exec(ctx, string(content)).ReadAll(); err != nil {
			_ = transaction.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", version, err)
		}

		if _, err := transaction.Exec(
			ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1)",
			version,
		); err != nil {
			_ = transaction.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", version, err)
		}

		if err := transaction.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", version, err)
		}
	}

	return nil
}

func sqlMigrations(files fs.FS, directory string) ([]string, error) {
	entries, err := fs.ReadDir(files, directory)
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}

	sort.Strings(names)
	return names, nil
}
