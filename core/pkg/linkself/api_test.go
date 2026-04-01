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

// TestSharedDB_NilBeforeStart: SharedDB() returns nil before Start.
func TestSharedDB_NilBeforeStart(t *testing.T) {
	c := NewClient()
	if c.SharedDB() != nil {
		t.Error("SharedDB() should return nil before Start")
	}
}

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

// TestSharedDB_AvailableAfterStart: GroupShare() returns non-nil after Start.
func TestSharedDB_AvailableAfterStart(t *testing.T) {
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

	if c.SharedDB() == nil {
		t.Error("GroupShare() should return non-nil after Start")
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

// TestSharedDB_RegisterAndPutGet: Register channel, put, get.
func TestSharedDB_RegisterAndPutGet(t *testing.T) {
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

	groupID, _ := groups.CreateGroup(ctx, []string{myDID, "did:key:other"})

	gs := c.SharedDB()
	if err := gs.RegisterChannel("notes", groupID); err != nil {
		t.Fatalf("RegisterChannel: %v", err)
	}

	if err := gs.Put(ctx, "notes", "", "note1", []byte(`{"text":"hello"}`)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	rec, err := gs.Get(ctx, "notes", "note1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil {
		t.Fatal("Get returned nil")
	}
	if string(rec.Body) != `{"text":"hello"}` {
		t.Errorf("Body = %q", rec.Body)
	}
	if rec.DID != myDID {
		t.Errorf("DID = %q, want %q", rec.DID, myDID)
	}
	if rec.Channel != "notes" {
		t.Errorf("Channel = %q, want notes", rec.Channel)
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

// TestSharedDB_GetNotFound: Get a non-existent record returns nil.
func TestSharedDB_GetNotFound(t *testing.T) {
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
	groupID, _ := groups.CreateGroup(ctx, []string{myDID, "did:key:other"})

	gs := c.SharedDB()
	_ = gs.RegisterChannel("notes", groupID)

	rec, err := gs.Get(ctx, "notes", "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec != nil {
		t.Error("Get should return nil for nonexistent record")
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

// TestSharedDB_DeleteAndList: Register, put, delete, list.
func TestSharedDB_DeleteAndList(t *testing.T) {
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
	groupID, _ := groups.CreateGroup(ctx, []string{myDID, "did:key:other"})

	gs := c.SharedDB()
	_ = gs.RegisterChannel("notes", groupID)
	_ = gs.Put(ctx, "notes", "", "n1", []byte(`a`))
	_ = gs.Put(ctx, "notes", "", "n2", []byte(`b`))

	recs, err := gs.List(ctx, "notes")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 2 {
		t.Errorf("List returned %d, want 2", len(recs))
	}

	_ = gs.Delete(ctx, "notes", "", "n1")
	recs, _ = gs.List(ctx, "notes")
	if len(recs) != 1 {
		t.Errorf("List after Delete = %d, want 1", len(recs))
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
