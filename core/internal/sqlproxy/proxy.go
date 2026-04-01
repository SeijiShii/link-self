// Package sqlproxy provides a SQL query interface that wraps SQLite and
// captures write operations for sync. Apps use standard SQL (Exec/Query/QueryRow);
// writes are detected and logged for the sync engine to broadcast.
package sqlproxy

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// WriteOp represents the type of a detected write operation.
type WriteOp int

const (
	OpInsert WriteOp = iota
	OpUpdate
	OpDelete
)

// WriteEntry records a detected write operation for sync.
type WriteEntry struct {
	Op  WriteOp
	SQL string
}

// Migration defines a schema migration step.
type Migration struct {
	Version int
	SQL     string
}

// Proxy wraps a SQLite database and intercepts write operations.
type Proxy struct {
	db       *sql.DB
	WriteLog []WriteEntry // captured writes (for sync engine to consume)
}

// Open creates a new Proxy backed by a SQLite database at the given path.
// Use ":memory:" for an in-memory database.
func Open(path string) (*Proxy, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Enable WAL mode for better concurrency.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	// Create migration tracking table.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS _migrations (version INTEGER PRIMARY KEY)`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create migrations table: %w", err)
	}
	return &Proxy{db: db}, nil
}

// Close closes the underlying database.
func (p *Proxy) Close() error {
	return p.db.Close()
}

// Exec executes a SQL statement. If the statement is a write (INSERT/UPDATE/DELETE),
// it is recorded in WriteLog for the sync engine.
func (p *Proxy) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	result, err := p.db.ExecContext(ctx, query, args...)
	if err != nil {
		return result, err
	}
	if op, ok := detectWrite(query); ok {
		p.WriteLog = append(p.WriteLog, WriteEntry{Op: op, SQL: query})
	}
	return result, nil
}

// Query executes a query that returns rows (SELECT).
func (p *Proxy) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return p.db.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row.
func (p *Proxy) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return p.db.QueryRowContext(ctx, query, args...)
}

// Migrate runs migrations in order, skipping already-applied versions.
func (p *Proxy) Migrate(ctx context.Context, migrations []Migration) error {
	for _, m := range migrations {
		var exists int
		err := p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM _migrations WHERE version = ?`, m.Version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %d: %w", m.Version, err)
		}
		if exists > 0 {
			continue
		}
		if _, err := p.db.ExecContext(ctx, m.SQL); err != nil {
			return fmt.Errorf("apply migration %d: %w", m.Version, err)
		}
		if _, err := p.db.ExecContext(ctx, `INSERT INTO _migrations (version) VALUES (?)`, m.Version); err != nil {
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}
	}
	return nil
}

// detectWrite checks if a SQL statement is a write operation.
func detectWrite(query string) (WriteOp, bool) {
	trimmed := strings.TrimSpace(strings.ToUpper(query))
	switch {
	case strings.HasPrefix(trimmed, "INSERT"):
		return OpInsert, true
	case strings.HasPrefix(trimmed, "UPDATE"):
		return OpUpdate, true
	case strings.HasPrefix(trimmed, "DELETE"):
		return OpDelete, true
	case strings.HasPrefix(trimmed, "REPLACE"):
		return OpInsert, true
	default:
		return 0, false
	}
}
