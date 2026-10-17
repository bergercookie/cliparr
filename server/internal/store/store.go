// Package store persists clips and login sessions in SQLite.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, keeps the binary CGO-free
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// ErrNotFound is returned when a row does not exist (or belongs to someone else).
var ErrNotFound = errors.New("store: not found")

// Store is the Cliparr database.
type Store struct {
	db *sql.DB
}

// Open opens (and migrates) the SQLite database at path.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// SQLite allows one writer; a single connection avoids "database is locked".
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// migrate applies migrations/NNN_*.sql in order, tracking progress in PRAGMA user_version.
func (s *Store) migrate(ctx context.Context) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	var current int
	if err := s.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	for i := current; i < len(names); i++ {
		sqlText, err := migrationFiles.ReadFile("migrations/" + names[i])
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(sqlText)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply %s: %w", names[i], err)
		}
		// PRAGMA does not accept bound parameters.
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// DeviceID returns the stable identifier Cliparr presents to Jellyfin,
// generating it on first use.
func (s *Store) DeviceID(ctx context.Context) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM meta WHERE key = 'device_id'").Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	id = "cliparr-" + hex.EncodeToString(raw)
	if _, err := s.db.ExecContext(ctx, "INSERT INTO meta (key, value) VALUES ('device_id', ?)", id); err != nil {
		return "", err
	}
	return id, nil
}

func encodeTags(tags []string) string {
	return "," + strings.Join(tags, ",") + ","
}

func decodeTags(s string) []string {
	trimmed := strings.Trim(s, ",")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, ",")
}
