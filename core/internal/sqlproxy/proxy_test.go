package sqlproxy

import (
	"context"
	"testing"
)

func TestProxy_Exec_CreateTable(t *testing.T) {
	p, cleanup := newTestProxy(t)
	defer cleanup()
	ctx := context.Background()

	_, err := p.Exec(ctx, `CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}
}

func TestProxy_Exec_InsertAndQuery(t *testing.T) {
	p, cleanup := newTestProxy(t)
	defer cleanup()
	ctx := context.Background()

	p.Exec(ctx, `CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT)`)

	_, err := p.Exec(ctx, `INSERT INTO users (id, name) VALUES (?, ?)`, "1", "Alice")
	if err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	rows, err := p.Query(ctx, `SELECT id, name FROM users WHERE id = ?`, "1")
	if err != nil {
		t.Fatalf("SELECT: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("expected 1 row")
	}
	var id, name string
	if err := rows.Scan(&id, &name); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if id != "1" || name != "Alice" {
		t.Errorf("got (%q, %q), want (1, Alice)", id, name)
	}
}

func TestProxy_QueryRow(t *testing.T) {
	p, cleanup := newTestProxy(t)
	defer cleanup()
	ctx := context.Background()

	p.Exec(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY, val INTEGER)`)
	p.Exec(ctx, `INSERT INTO items (id, val) VALUES (?, ?)`, "a", 42)

	var val int
	err := p.QueryRow(ctx, `SELECT val FROM items WHERE id = ?`, "a").Scan(&val)
	if err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	if val != 42 {
		t.Errorf("val = %d, want 42", val)
	}
}

func TestProxy_Exec_UpdateAndDelete(t *testing.T) {
	p, cleanup := newTestProxy(t)
	defer cleanup()
	ctx := context.Background()

	p.Exec(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY, val TEXT)`)
	p.Exec(ctx, `INSERT INTO items (id, val) VALUES (?, ?)`, "a", "old")

	_, err := p.Exec(ctx, `UPDATE items SET val = ? WHERE id = ?`, "new", "a")
	if err != nil {
		t.Fatalf("UPDATE: %v", err)
	}

	var val string
	p.QueryRow(ctx, `SELECT val FROM items WHERE id = ?`, "a").Scan(&val)
	if val != "new" {
		t.Errorf("val = %q, want new", val)
	}

	_, err = p.Exec(ctx, `DELETE FROM items WHERE id = ?`, "a")
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}

	rows, _ := p.Query(ctx, `SELECT id FROM items`)
	defer rows.Close()
	if rows.Next() {
		t.Error("expected 0 rows after DELETE")
	}
}

func TestProxy_WriteDetection(t *testing.T) {
	p, cleanup := newTestProxy(t)
	defer cleanup()
	ctx := context.Background()

	p.Exec(ctx, `CREATE TABLE docs (id TEXT PRIMARY KEY, body TEXT)`)
	p.Exec(ctx, `INSERT INTO docs (id, body) VALUES (?, ?)`, "d1", "hello")
	p.Exec(ctx, `UPDATE docs SET body = ? WHERE id = ?`, "world", "d1")
	p.Exec(ctx, `DELETE FROM docs WHERE id = ?`, "d1")

	// WriteLog should have captured 3 writes (INSERT, UPDATE, DELETE)
	// CREATE TABLE is DDL, not a data write
	if len(p.WriteLog) != 3 {
		t.Errorf("WriteLog has %d entries, want 3", len(p.WriteLog))
	}
	if p.WriteLog[0].Op != OpInsert {
		t.Errorf("entry 0 op = %v, want Insert", p.WriteLog[0].Op)
	}
	if p.WriteLog[1].Op != OpUpdate {
		t.Errorf("entry 1 op = %v, want Update", p.WriteLog[1].Op)
	}
	if p.WriteLog[2].Op != OpDelete {
		t.Errorf("entry 2 op = %v, want Delete", p.WriteLog[2].Op)
	}
}

func TestProxy_Migrate(t *testing.T) {
	p, cleanup := newTestProxy(t)
	defer cleanup()
	ctx := context.Background()

	err := p.Migrate(ctx, []Migration{
		{Version: 1, SQL: `CREATE TABLE v1 (id TEXT PRIMARY KEY)`},
		{Version: 2, SQL: `CREATE TABLE v2 (id TEXT PRIMARY KEY, name TEXT)`},
	})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Tables should exist
	p.Exec(ctx, `INSERT INTO v1 (id) VALUES ('a')`)
	p.Exec(ctx, `INSERT INTO v2 (id, name) VALUES ('b', 'Bob')`)

	// Re-running same migrations should be idempotent
	err = p.Migrate(ctx, []Migration{
		{Version: 1, SQL: `CREATE TABLE v1 (id TEXT PRIMARY KEY)`},
		{Version: 2, SQL: `CREATE TABLE v2 (id TEXT PRIMARY KEY, name TEXT)`},
	})
	if err != nil {
		t.Fatalf("Migrate (idempotent): %v", err)
	}
}

// newTestProxy creates an in-memory SQLite proxy for testing.
func newTestProxy(t *testing.T) (*Proxy, func()) {
	t.Helper()
	p, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return p, func() { p.Close() }
}
