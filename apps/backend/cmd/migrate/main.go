package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	_ "github.com/lib/pq"
)

func main() {
	driver := envOrDefault("OPS_API_DB_DRIVER", "postgres")
	dsn := os.Getenv("OPS_API_DB_DSN")
	if dsn == "" {
		log.Fatal("OPS_API_DB_DSN is required")
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	files, err := findMigrationFiles()
	if err != nil {
		log.Fatalf("failed to find migrations: %v", err)
	}

	ctx := context.Background()
	if err := ensureSchemaMigrations(ctx, db); err != nil {
		log.Fatalf("failed to ensure schema_migrations table: %v", err)
	}

	applied, err := loadAppliedMigrations(ctx, db)
	if err != nil {
		log.Fatalf("failed to load applied migrations: %v", err)
	}

	appliedCount := 0
	for _, file := range files {
		if _, ok := applied[filepath.Base(file)]; ok {
			log.Printf("skipping already applied migration: %s", file)
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("failed to read migration %s: %v", file, err)
		}

		log.Printf("applying migration: %s", file)
		if err := applyMigration(ctx, db, filepath.Base(file), string(content)); err != nil {
			log.Fatalf("failed to execute migration %s: %v", file, err)
		}
		appliedCount++
	}

	log.Printf("applied %d migration(s)", appliedCount)
}

func findMigrationFiles() ([]string, error) {
	patterns := []string{"migrations/*.sql", "apps/backend/migrations/*.sql"}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		if len(matches) > 0 {
			sort.Strings(matches)
			return matches, nil
		}
	}
	return nil, errors.New("no migration files found")
}

func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func ensureSchemaMigrations(ctx context.Context, db *sql.DB) error {
	const q = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  filename TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
`

	_, err := db.ExecContext(ctx, q)
	return err
}

func loadAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `SELECT filename FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]struct{})
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return nil, err
		}
		out[filename] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func applyMigration(ctx context.Context, db *sql.DB, filename, content string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, content); err != nil {
		_ = tx.Rollback()
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (filename, applied_at) VALUES ($1, NOW())`,
		filename,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration %s: %w", filename, err)
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
