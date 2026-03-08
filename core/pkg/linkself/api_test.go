package linkself

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// TestDeviceDB_NilBeforeStart: DeviceDB() returns nil before Start.
func TestDeviceDB_NilBeforeStart(t *testing.T) {
	c := NewClient()
	if c.DeviceDB() != nil {
		t.Error("DeviceDB() should return nil before Start")
	}
}

// TestGroupShare_NilBeforeStart: GroupShare() returns nil before Start.
func TestGroupShare_NilBeforeStart(t *testing.T) {
	c := NewClient()
	if c.GroupShare() != nil {
		t.Error("GroupShare() should return nil before Start")
	}
}

// TestGroups_NilBeforeStart: Groups() returns nil before Start.
func TestGroups_NilBeforeStart(t *testing.T) {
	c := NewClient()
	if c.Groups() != nil {
		t.Error("Groups() should return nil before Start")
	}
}

// TestDeviceDB_AvailableAfterStart: DeviceDB() returns non-nil after Start.
func TestDeviceDB_AvailableAfterStart(t *testing.T) {
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

	if c.DeviceDB() == nil {
		t.Error("DeviceDB() should return non-nil after Start")
	}
}

// TestGroupShare_AvailableAfterStart: GroupShare() returns non-nil after Start.
func TestGroupShare_AvailableAfterStart(t *testing.T) {
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

	if c.GroupShare() == nil {
		t.Error("GroupShare() should return non-nil after Start")
	}
}

// TestGroups_AvailableAfterStart: Groups() returns non-nil after Start.
func TestGroups_AvailableAfterStart(t *testing.T) {
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

	if c.Groups() == nil {
		t.Error("Groups() should return non-nil after Start")
	}
}

// TestDeviceDB_PutGet: Put a record then Get it back.
func TestDeviceDB_PutGet(t *testing.T) {
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

	db := c.DeviceDB()

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

// TestDeviceDB_Delete: Put then Delete; Get returns nil.
func TestDeviceDB_Delete(t *testing.T) {
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

	db := c.DeviceDB()

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

// TestDeviceDB_List: Put multiple records; List returns them.
func TestDeviceDB_List(t *testing.T) {
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

	db := c.DeviceDB()

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

// TestGroups_CreateAndList: Create a group and list it.
func TestGroups_CreateAndList(t *testing.T) {
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

	groups := c.Groups()
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

// TestGroups_Leave: Create a group, then leave it.
func TestGroups_Leave(t *testing.T) {
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

	groups := c.Groups()
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

// TestGroupShare_RegisterAndPutGet: Register channel, put, get.
func TestGroupShare_RegisterAndPutGet(t *testing.T) {
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

	groups := c.Groups()
	myDID := c.GetMyDID()

	groupID, _ := groups.CreateGroup(ctx, []string{myDID, "did:key:other"})

	gs := c.GroupShare()
	if err := gs.RegisterChannel("notes", groupID); err != nil {
		t.Fatalf("RegisterChannel: %v", err)
	}

	if err := gs.Put(ctx, "notes", "note1", []byte(`{"text":"hello"}`)); err != nil {
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

// TestGroups_AddMember: Create a group, then add a member.
func TestGroups_AddMember(t *testing.T) {
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

	groups := c.Groups()
	myDID := c.GetMyDID()

	groupID, _ := groups.CreateGroup(ctx, []string{myDID, "did:key:bob"})
	if err := groups.AddMember(ctx, groupID, "did:key:carol"); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
}

// TestGroupShare_GetNotFound: Get a non-existent record returns nil.
func TestGroupShare_GetNotFound(t *testing.T) {
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

	groups := c.Groups()
	myDID := c.GetMyDID()
	groupID, _ := groups.CreateGroup(ctx, []string{myDID, "did:key:other"})

	gs := c.GroupShare()
	_ = gs.RegisterChannel("notes", groupID)

	rec, err := gs.Get(ctx, "notes", "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec != nil {
		t.Error("Get should return nil for nonexistent record")
	}
}

// TestDeviceDB_GetNotFound: Get a non-existent record returns nil.
func TestDeviceDB_GetNotFound(t *testing.T) {
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

	db := c.DeviceDB()
	rec, err := db.Get(ctx, "contacts", "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec != nil {
		t.Error("Get should return nil for nonexistent record")
	}
}

// TestGroupShare_DeleteAndList: Register, put, delete, list.
func TestGroupShare_DeleteAndList(t *testing.T) {
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

	groups := c.Groups()
	myDID := c.GetMyDID()
	groupID, _ := groups.CreateGroup(ctx, []string{myDID, "did:key:other"})

	gs := c.GroupShare()
	_ = gs.RegisterChannel("notes", groupID)
	_ = gs.Put(ctx, "notes", "n1", []byte(`a`))
	_ = gs.Put(ctx, "notes", "n2", []byte(`b`))

	recs, err := gs.List(ctx, "notes")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 2 {
		t.Errorf("List returned %d, want 2", len(recs))
	}

	_ = gs.Delete(ctx, "notes", "n1")
	recs, _ = gs.List(ctx, "notes")
	if len(recs) != 1 {
		t.Errorf("List after Delete = %d, want 1", len(recs))
	}
}
