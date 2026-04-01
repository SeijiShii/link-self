// Package sqlite provides SQLite3-backed implementations of all LinkSelf
// storage interfaces. It uses a single WAL-mode database file and manages
// schema migrations via PRAGMA user_version.
package sqlite

import (
	"database/sql"
	"fmt"

	_ "github.com/ncruces/go-sqlite3/driver"

	"github.com/SeijiShii/link-self/core/internal/devicesync"
	"github.com/SeijiShii/link-self/core/internal/group"
	"github.com/SeijiShii/link-self/core/internal/groupshare"
	"github.com/SeijiShii/link-self/core/internal/network"
	"github.com/SeijiShii/link-self/core/internal/syncdb"
)

// DB wraps a single SQLite3 connection and exposes typed storage accessors.
type DB struct {
	conn *sql.DB
}

// Open opens (or creates) a SQLite3 database at path, enables WAL mode,
// and runs any pending schema migrations. Use ":memory:" for testing.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	// Single connection to avoid locking issues with WAL.
	conn.SetMaxOpenConns(1)

	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("sqlite WAL: %w", err)
	}
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("sqlite FK: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("sqlite migrate: %w", err)
	}
	return db, nil
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// RecordStorage returns a syncdb.RecordStorage backed by this DB.
func (db *DB) RecordStorage() syncdb.RecordStorage {
	return &recordStorage{db: db.conn}
}

// SharedStorage returns a groupshare.SharedStorage backed by this DB.
func (db *DB) SharedStorage() groupshare.SharedStorage {
	return &sharedStorage{db: db.conn}
}

// DeviceStorage returns a devicesync.DeviceStorage backed by this DB.
func (db *DB) DeviceStorage() devicesync.DeviceStorage {
	return &deviceStorage{db: db.conn}
}

// SubscriptionStore returns a groupshare.SubscriptionStore backed by this DB.
func (db *DB) SubscriptionStore() groupshare.SubscriptionStore {
	return &subscriptionStore{db: db.conn}
}

// GroupStore returns a group.Store backed by this DB.
func (db *DB) GroupStore() group.Store {
	return &groupStore{db: db.conn}
}

// NetworkStore returns a network.Store backed by this DB.
func (db *DB) NetworkStore() network.Store {
	return &networkStore{db: db.conn}
}
