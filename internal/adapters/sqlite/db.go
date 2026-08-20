// Package sqlite is the secondary adapter that persists job state. It is the
// only package in the tree allowed to import database/sql.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite" // pure-Go driver: keeps builds static, no cgo
)

// migrationFiles embeds the schema so it ships inside the binary and applies
// itself on startup — no external migration tool needed at this scale
// (AGENTS.md 4.4).
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// Open opens the job database and applies the schema.
//
// path is a file path ("do0ps.db"); the pragmas below turn on WAL for
// concurrent readers and make writers wait briefly instead of failing with
// SQLITE_BUSY.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating database directory %q: %w", dir, err)
		}
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite database %q: %w", path, err)
	}

	// SQLite serializes writes; a large pool only creates lock contention.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to sqlite database %q: %w", path, err)
	}

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// migrate applies every embedded *.sql file in migrations/, in filename
// order, each within its own transaction. Files are idempotent (CREATE
// TABLE/INDEX IF NOT EXISTS), so re-running an already-applied migration on
// every startup is safe and needs no separate "applied migrations" ledger.
func migrate(ctx context.Context, db *sql.DB) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading embedded migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		stmt, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("reading migration %q: %w", name, err)
		}
		if _, err := db.ExecContext(ctx, string(stmt)); err != nil {
			return fmt.Errorf("applying migration %q: %w", name, err)
		}
	}
	return nil
}
