package linkself

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestMyDB_NilBeforeStart: MyDB() returns nil before Start.
func TestMyDB_NilBeforeStart(t *testing.T) {
	c := NewClient()
	if c.MyDB() != nil {
		t.Error("MyDB() should return nil before Start")
	}
}

// Note: SharedDB tests removed — SharedDB is no longer a public API.

// TestNetwork_NilBeforeStart: Network() returns nil before Start.
func TestNetwork_NilBeforeStart(t *testing.T) {
	c := NewClient()
	if c.Network() != nil {
		t.Error("Network() should return nil before Start")
	}
}

// TestMyDB_AvailableAfterStart: DeviceDB() returns non-nil after Start.
func TestMyDB_AvailableAfterStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	if c.MyDB() == nil {
		t.Error("DeviceDB() should return non-nil after Start")
	}
}


// TestNetwork_AvailableAfterStart: Groups() returns non-nil after Start.
func TestNetwork_AvailableAfterStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	if c.Network() == nil {
		t.Error("Groups() should return non-nil after Start")
	}
}

// TestMyDB_PutGet: Put a record then Get it back.
func TestMyDB_PutGet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	db := c.MyDB()

	// Put
	if err := db.Put(ctx, "contacts", "alice", []byte(`{"name":"Alice"}`)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Get
	rec, err := db.Get(ctx, "contacts", "alice")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil {
		t.Fatal("Get returned nil")
	}
	if string(rec.Body) != `{"name":"Alice"}` {
		t.Errorf("Body = %q, want {\"name\":\"Alice\"}", rec.Body)
	}
	if rec.ID != "alice" {
		t.Errorf("ID = %q, want alice", rec.ID)
	}
	if rec.Table != "contacts" {
		t.Errorf("Table = %q, want contacts", rec.Table)
	}
}

// TestMyDB_Delete: Put then Delete; Get returns nil.
func TestMyDB_Delete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	db := c.MyDB()

	_ = db.Put(ctx, "contacts", "alice", []byte(`{"name":"Alice"}`))
	if err := db.Delete(ctx, "contacts", "alice"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	rec, err := db.Get(ctx, "contacts", "alice")
	if err != nil {
		t.Fatalf("Get after Delete: %v", err)
	}
	if rec != nil {
		t.Error("Get after Delete should return nil")
	}
}

// TestMyDB_List: Put multiple records; List returns them.
func TestMyDB_List(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	db := c.MyDB()

	_ = db.Put(ctx, "contacts", "alice", []byte(`{"name":"Alice"}`))
	_ = db.Put(ctx, "contacts", "bob", []byte(`{"name":"Bob"}`))

	recs, err := db.List(ctx, "contacts")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 2 {
		t.Errorf("List returned %d records, want 2", len(recs))
	}
}

// TestNetwork_CreateAndList: Create a group and list it.
func TestNetwork_CreateAndList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	groups := c.Network()
	myDID := c.GetMyDID()

	groupID, err := groups.CreateGroup(ctx, []string{myDID, "did:key:other-member"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if groupID == "" {
		t.Fatal("CreateGroup returned empty groupID")
	}

	ids, err := groups.ListGroups(ctx)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(ids) != 1 || ids[0] != groupID {
		t.Errorf("ListGroups = %v, want [%s]", ids, groupID)
	}
}

// TestNetwork_Leave: Create a group, then leave it.
func TestNetwork_Leave(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	groups := c.Network()
	myDID := c.GetMyDID()

	groupID, _ := groups.CreateGroup(ctx, []string{myDID, "did:key:other-member"})
	if err := groups.Leave(ctx, groupID); err != nil {
		t.Fatalf("Leave: %v", err)
	}

	ids, _ := groups.ListGroups(ctx)
	if len(ids) != 0 {
		t.Errorf("ListGroups after Leave = %v, want empty", ids)
	}
}


// TestNetwork_AddMember: Create a group, then add a member.
func TestNetwork_AddMember(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	groups := c.Network()
	myDID := c.GetMyDID()

	groupID, _ := groups.CreateGroup(ctx, []string{myDID, "did:key:bob"})
	if err := groups.AddMember(ctx, groupID, "did:key:carol"); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
}


// TestMyDB_GetNotFound: Get a non-existent record returns nil.
func TestMyDB_GetNotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	db := c.MyDB()
	rec, err := db.Get(ctx, "contacts", "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec != nil {
		t.Error("Get should return nil for nonexistent record")
	}
}


// TestMyDB_ExecAndQuery: Create table, insert, query via MyDB SQL interface.
func TestMyDB_ExecAndQuery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	db := c.MyDB()

	_, err = db.Exec(ctx, `CREATE TABLE items (id TEXT PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("CREATE TABLE: %v", err)
	}

	_, err = db.Exec(ctx, `INSERT INTO items (id, name) VALUES (?, ?)`, "1", "Alice")
	if err != nil {
		t.Fatalf("INSERT: %v", err)
	}

	rows, err := db.Query(ctx, `SELECT id, name FROM items`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("expected 1 row")
	}
	var id, name string
	rows.Scan(&id, &name)
	if id != "1" || name != "Alice" {
		t.Errorf("got (%q, %q), want (1, Alice)", id, name)
	}
}

// TestMyDB_Migrate: Migrate creates tables across versions.
func TestMyDB_Migrate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	db := c.MyDB()
	err = db.Migrate(ctx, []Migration{
		{Version: 1, SQL: `CREATE TABLE docs (id TEXT PRIMARY KEY, body TEXT)`},
	})
	if err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	_, err = db.Exec(ctx, `INSERT INTO docs (id, body) VALUES (?, ?)`, "d1", "hello")
	if err != nil {
		t.Fatalf("INSERT after Migrate: %v", err)
	}
}

// TestMyDB_SetSyncScope_Default: Default scope is ScopeDevice.
func TestMyDB_SetSyncScope_Default(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	db := c.MyDB()

	// SetSyncScope to ScopeDevice should succeed (it's the default, no-op).
	err = db.SetSyncScope(ctx, "notes", ScopeDevice)
	if err != nil {
		t.Fatalf("SetSyncScope(ScopeDevice): %v", err)
	}
}

// TestMyDB_SetSyncScope_Network: Set scope to ScopeNetwork.
func TestMyDB_SetSyncScope_Network(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	db := c.MyDB()

	// SetSyncScope to ScopeNetwork should succeed.
	err = db.SetSyncScope(ctx, "notes", ScopeNetwork)
	if err != nil {
		t.Fatalf("SetSyncScope(ScopeNetwork): %v", err)
	}
}

// TestMyDB_SetSyncScope_WithIncludeExisting: IncludeExisting option is accepted.
func TestMyDB_SetSyncScope_WithIncludeExisting(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	db := c.MyDB()

	// Put some data first
	_ = db.Put(ctx, "notes", "n1", []byte(`{"text":"hello"}`))

	// SetSyncScope with IncludeExisting should succeed.
	err = db.SetSyncScope(ctx, "notes", ScopeNetwork, WithIncludeExisting(true))
	if err != nil {
		t.Fatalf("SetSyncScope with IncludeExisting: %v", err)
	}
}

// TestMyDB_SQLWrite_SyncsViaDeviceSync: SQL INSERT triggers DeviceSync replication.
func TestMyDB_SQLWrite_SyncsViaDeviceSync(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	db := c.MyDB()
	db.Exec(ctx, `CREATE TABLE notes (id TEXT PRIMARY KEY, body TEXT)`)
	db.Exec(ctx, `INSERT INTO notes (id, body) VALUES (?, ?)`, "n1", "hello")

	// The write should have been captured by the DeviceSync engine's ChangeLog.
	// Verify via the KV layer (the ReplicationEngine's storage).
	rec, err := db.Get(ctx, "notes", "n1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	// Note: until C-3 wiring is done, SQL writes won't appear in KV storage.
	// This test will FAIL (Red) until we connect OnWriteEvent → engine.Put.
	if rec == nil {
		t.Fatal("SQL INSERT should be visible via KV Get after sync wiring")
	}
}

// TestClient_Start_WithSuiteID: SuiteID creates storage in suite directory.
func TestClient_Start_WithSuiteID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dataDir := t.TempDir()
	t.Setenv("LINKSELF_DATA_DIR", dataDir)

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
		SuiteID:      "com.example.notes",
	}
	info, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	// Verify we can use SQL (proves SQLite was opened at a real path).
	db := c.MyDB()
	_, err = db.Exec(ctx, `CREATE TABLE t (id TEXT PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}

	// Verify the suite directory was created under the data root.
	// Path: <dataDir>/<encodedDID>/suites/com.example.notes/
	did := info.DID
	entries, _ := filepath.Glob(filepath.Join(dataDir, "*", "suites", "com.example.notes"))
	if len(entries) == 0 {
		t.Errorf("suite directory not found for DID %s under %s", did, dataDir)
	}
}

// TestClient_Start_WithoutSuiteID: No SuiteID uses in-memory storage.
func TestClient_Start_WithoutSuiteID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
		// No SuiteID — should still work (in-memory).
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	// Should still work with in-memory DB.
	db := c.MyDB()
	_, err = db.Exec(ctx, `CREATE TABLE t (id TEXT PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("Exec without SuiteID: %v", err)
	}
}

// TestMyDB_DumpRestore: Dump all records, then restore to a fresh client.
func TestMyDB_DumpRestore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	db := c.MyDB()

	// Put some records in two tables
	_ = db.Put(ctx, "contacts", "alice", []byte(`{"name":"Alice"}`))
	_ = db.Put(ctx, "contacts", "bob", []byte(`{"name":"Bob"}`))
	_ = db.Put(ctx, "notes", "n1", []byte(`{"text":"hello"}`))

	// Dump
	records, err := db.Dump(ctx)
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("Dump returned %d records, want 3", len(records))
	}

	// Start a fresh client and restore
	c2 := NewClient()
	config2 := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity2.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err = c2.Start(ctx, config2)
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Stop(ctx)

	db2 := c2.MyDB()
	applied, err := db2.Restore(ctx, records)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if applied != 3 {
		t.Errorf("Restore applied %d, want 3", applied)
	}

	// Verify restored data
	rec, _ := db2.Get(ctx, "contacts", "alice")
	if rec == nil {
		t.Fatal("restored alice not found")
	}
	if string(rec.Body) != `{"name":"Alice"}` {
		t.Errorf("alice Body = %q", rec.Body)
	}

	recs, _ := db2.List(ctx, "contacts")
	if len(recs) != 2 {
		t.Errorf("contacts count = %d, want 2", len(recs))
	}
}

// TestMyDB_DumpEmpty: Dump on empty DB returns empty slice.
func TestMyDB_DumpEmpty(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	records, err := c.MyDB().Dump(ctx)
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("Dump on empty DB returned %d records, want 0", len(records))
	}
}

// TestMyDB_RestoreLWW: Restore skips older records.
func TestMyDB_RestoreLWW(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	_, err := c.Start(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Stop(ctx)

	db := c.MyDB()

	// Put a record (will have current timestamp)
	_ = db.Put(ctx, "contacts", "alice", []byte(`{"name":"Alice-new"}`))

	// Try to restore an older version
	oldRecords := []*Record{
		{ID: "alice", Table: "contacts", Body: []byte(`{"name":"Alice-old"}`), Timestamp: 1},
	}
	applied, err := db.Restore(ctx, oldRecords)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if applied != 0 {
		t.Errorf("Restore applied %d, want 0 (older record should be skipped)", applied)
	}

	// Original should be unchanged
	rec, _ := db.Get(ctx, "contacts", "alice")
	if string(rec.Body) != `{"name":"Alice-new"}` {
		t.Errorf("Body = %q, want Alice-new", rec.Body)
	}
}
