package sqlite

import "fmt"

// schemaVersion is the current schema version.
const schemaVersion = 2

// migrations maps from version → DDL that upgrades TO that version.
var migrations = map[int]string{
	1: `
CREATE TABLE IF NOT EXISTS sync_records (
    id        TEXT PRIMARY KEY,
    group_id  TEXT NOT NULL,
    did       TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    body      BLOB,
    deleted   INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sync_records_group ON sync_records(group_id);

CREATE TABLE IF NOT EXISTS shared_records (
    channel   TEXT NOT NULL,
    id        TEXT NOT NULL,
    topic     TEXT NOT NULL DEFAULT '',
    group_id  TEXT NOT NULL,
    did       TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    body      BLOB,
    deleted   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (channel, id)
);
CREATE INDEX IF NOT EXISTS idx_shared_channel_topic ON shared_records(channel, topic);
CREATE INDEX IF NOT EXISTS idx_shared_group ON shared_records(group_id);
CREATE INDEX IF NOT EXISTS idx_shared_timestamp ON shared_records(channel, timestamp);

CREATE TABLE IF NOT EXISTS device_records (
    tbl       TEXT NOT NULL,
    id        TEXT NOT NULL,
    body      BLOB,
    timestamp INTEGER NOT NULL,
    PRIMARY KEY (tbl, id)
);

CREATE TABLE IF NOT EXISTS change_log (
    seq       INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp INTEGER NOT NULL,
    tbl       TEXT NOT NULL,
    record_id TEXT NOT NULL,
    op        INTEGER NOT NULL,
    body      BLOB
);

CREATE TABLE IF NOT EXISTS groups_ (
    id      TEXT PRIMARY KEY,
    members TEXT NOT NULL DEFAULT '[]',
    owners  TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS subscriptions (
    did     TEXT NOT NULL,
    channel TEXT NOT NULL,
    topics  TEXT NOT NULL DEFAULT '[]',
    PRIMARY KEY (did, channel)
);
`,
	2: `
CREATE TABLE IF NOT EXISTS networks (
    id           TEXT PRIMARY KEY,
    suite_id     TEXT NOT NULL DEFAULT '',
    members      TEXT NOT NULL DEFAULT '[]',
    member_roles TEXT NOT NULL DEFAULT '{}'
);
`,
}

func (db *DB) migrate() error {
	var current int
	if err := db.conn.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}

	for v := current + 1; v <= schemaVersion; v++ {
		ddl, ok := migrations[v]
		if !ok {
			return fmt.Errorf("missing migration for version %d", v)
		}
		if _, err := db.conn.Exec(ddl); err != nil {
			return fmt.Errorf("migration v%d: %w", v, err)
		}
		if _, err := db.conn.Exec(fmt.Sprintf("PRAGMA user_version = %d", v)); err != nil {
			return fmt.Errorf("set user_version %d: %w", v, err)
		}
	}
	return nil
}
