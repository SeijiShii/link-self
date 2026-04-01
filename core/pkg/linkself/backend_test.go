package linkself

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// helper: start a client in tempDir with the given backend (nil = default memory).
func startClient(t *testing.T, tempDir string, backend StorageBackend) (Client, context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	c := NewClient()
	cfg := Config{
		IdentityPath:   filepath.Join(tempDir, "identity.json"),
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		StorageBackend: backend,
	}
	if _, err := c.Start(ctx, cfg); err != nil {
		cancel()
		t.Fatalf("Start: %v", err)
	}
	return c, ctx, cancel
}

// TestMemoryBackend_NonNil: MemoryBackend() returns a non-nil StorageBackend.
func TestMemoryBackend_NonNil(t *testing.T) {
	b := MemoryBackend()
	if b == nil {
		t.Fatal("MemoryBackend() returned nil")
	}
}

// TestSQLiteBackend_NonNil: SQLiteBackend(path) returns a non-nil StorageBackend.
func TestSQLiteBackend_NonNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	b := SQLiteBackend(path)
	if b == nil {
		t.Fatal("SQLiteBackend() returned nil")
	}
}

// TestStorageBackend_OpaqueType: StorageBackend is opaque — it cannot be
// implemented outside the package. We verify it has the unexported marker method
// by checking that external types cannot satisfy it. This is a compile-time
// guarantee; here we just confirm MemoryBackend() is assignable to StorageBackend.
func TestStorageBackend_OpaqueType(t *testing.T) {
	var _ StorageBackend = MemoryBackend()
	var _ StorageBackend = SQLiteBackend(filepath.Join(t.TempDir(), "x.db"))
}

// TestNilBackend_BackwardCompat: Config with nil StorageBackend starts successfully.
func TestNilBackend_BackwardCompat(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
		// StorageBackend intentionally nil
	}

	info, err := c.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start with nil backend failed: %v", err)
	}
	if info == nil {
		t.Fatal("Start returned nil NodeInfo")
	}
	if err := c.Stop(ctx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

// TestMemoryBackend_ClientStartStop: Client starts and stops cleanly with MemoryBackend.
func TestMemoryBackend_ClientStartStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath:   filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		StorageBackend: MemoryBackend(),
	}

	info, err := c.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if info.DID == "" {
		t.Error("DID is empty")
	}
	if err := c.Stop(ctx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

// TestSQLiteBackend_ClientStartStop: Client starts and stops cleanly with SQLiteBackend.
func TestSQLiteBackend_ClientStartStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	c := NewClient()
	config := Config{
		IdentityPath:   filepath.Join(tempDir, "identity.json"),
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		StorageBackend: SQLiteBackend(filepath.Join(tempDir, "data.db")),
	}

	info, err := c.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if info.DID == "" {
		t.Error("DID is empty")
	}
	if err := c.Stop(ctx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

// TestSQLiteBackend_DataSurvivesRestart: Data written with SQLite backend persists
// across client Stop/Start cycles (same DB file).
func TestSQLiteBackend_DataSurvivesRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	identityPath := filepath.Join(tempDir, "identity.json")
	dbPath := filepath.Join(tempDir, "data.db")

	// First client: write a record.
	c1 := NewClient()
	config1 := Config{
		IdentityPath:   identityPath,
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		StorageBackend: SQLiteBackend(dbPath),
	}
	_, err := c1.Start(ctx, config1)
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	db1 := c1.MyDB()
	if err := db1.Put(ctx, "notes", "n1", []byte(`{"text":"hello"}`)); err != nil {
		t.Fatalf("Put: %v", err)
	}

	if err := c1.Stop(ctx); err != nil {
		t.Fatalf("First Stop failed: %v", err)
	}

	// Second client: read back the record using the same DB.
	c2 := NewClient()
	config2 := Config{
		IdentityPath:   identityPath,
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		StorageBackend: SQLiteBackend(dbPath),
	}
	_, err = c2.Start(ctx, config2)
	if err != nil {
		t.Fatalf("Second Start failed: %v", err)
	}
	defer c2.Stop(ctx)

	db2 := c2.MyDB()
	rec, err := db2.Get(ctx, "notes", "n1")
	if err != nil {
		t.Fatalf("Get after restart: %v", err)
	}
	if rec == nil {
		t.Fatal("Get after restart returned nil — data was not persisted")
	}
	if string(rec.Body) != `{"text":"hello"}` {
		t.Errorf("Body = %q, want {\"text\":\"hello\"}", rec.Body)
	}
}

// TestSQLiteBackend_StopClosesDB: Calling Stop() after SQLiteBackend does not leak
// the DB connection. We verify by starting a second client on the same path.
func TestSQLiteBackend_StopClosesDB(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	identityPath := filepath.Join(tempDir, "identity.json")
	dbPath := filepath.Join(tempDir, "data.db")

	c1 := NewClient()
	config := Config{
		IdentityPath:   identityPath,
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		StorageBackend: SQLiteBackend(dbPath),
	}
	if _, err := c1.Start(ctx, config); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := c1.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// Should be able to open the same DB again without error.
	c2 := NewClient()
	config2 := Config{
		IdentityPath:   identityPath,
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		StorageBackend: SQLiteBackend(dbPath),
	}
	if _, err := c2.Start(ctx, config2); err != nil {
		t.Fatalf("Second Start after Stop: %v — DB connection may not have been closed", err)
	}
	if err := c2.Stop(ctx); err != nil {
		t.Errorf("Second Stop: %v", err)
	}
}

// TestMemoryBackend_PutGet: Data written with MemoryBackend is readable in the same session.
func TestMemoryBackend_PutGet(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	config := Config{
		IdentityPath:   filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		StorageBackend: MemoryBackend(),
	}
	if _, err := c.Start(ctx, config); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop(ctx)

	db := c.MyDB()
	if err := db.Put(ctx, "items", "i1", []byte(`{"v":1}`)); err != nil {
		t.Fatalf("Put: %v", err)
	}
	rec, err := db.Get(ctx, "items", "i1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil {
		t.Fatal("Get returned nil")
	}
	if string(rec.Body) != `{"v":1}` {
		t.Errorf("Body = %q, want {\"v\":1}", rec.Body)
	}
}

// TestSQLiteBackend_GroupSurvivesRestart: Groups created with SQLiteBackend persist
// across client restart.
func TestSQLiteBackend_GroupSurvivesRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	identityPath := filepath.Join(tempDir, "identity.json")
	dbPath := filepath.Join(tempDir, "data.db")

	// First client: create a group.
	c1 := NewClient()
	_, err := c1.Start(ctx, Config{
		IdentityPath:   identityPath,
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		StorageBackend: SQLiteBackend(dbPath),
	})
	if err != nil {
		t.Fatalf("First Start: %v", err)
	}

	myDID := c1.GetMyDID()
	groups1 := c1.Network()
	groupID, err := groups1.CreateGroup(ctx, []string{myDID, "did:key:peer"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	if err := c1.Stop(ctx); err != nil {
		t.Fatalf("First Stop: %v", err)
	}

	// Second client: verify the group persisted.
	c2 := NewClient()
	_, err = c2.Start(ctx, Config{
		IdentityPath:   identityPath,
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		StorageBackend: SQLiteBackend(dbPath),
	})
	if err != nil {
		t.Fatalf("Second Start: %v", err)
	}
	defer c2.Stop(ctx)

	ids, err := c2.Network().ListGroups(ctx)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	found := false
	for _, id := range ids {
		if id == groupID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("group %q not found after restart; got %v", groupID, ids)
	}
}

// Note: SharedDB tests (Subscribe, DumpAndRestore, Purge, WithRetention)
// removed — SharedDB is no longer a public API.

// TestGetMyDID_AfterStop: GetMyDID returns empty string after Stop.
func TestGetMyDID_AfterStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	cfg := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	if _, err := c.Start(ctx, cfg); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if c.GetMyDID() == "" {
		t.Error("GetMyDID should return non-empty string while running")
	}
	c.Stop(ctx)
	if c.GetMyDID() != "" {
		t.Error("GetMyDID should return empty string after Stop")
	}
}

// TestStop_Idempotent: Calling Stop twice does not panic or return an error.
func TestStop_Idempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	cfg := Config{
		IdentityPath: filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}
	if _, err := c.Start(ctx, cfg); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := c.Stop(ctx); err != nil {
		t.Errorf("First Stop: %v", err)
	}
	if err := c.Stop(ctx); err != nil {
		t.Errorf("Second Stop: %v", err)
	}
}

// TestSQLiteBackend_InvalidPath: SQLiteBackend with an invalid path returns error on Start.
func TestSQLiteBackend_InvalidPath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := NewClient()
	cfg := Config{
		IdentityPath:   filepath.Join(t.TempDir(), "identity.json"),
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		StorageBackend: SQLiteBackend("/this/path/does/not/exist/db.sqlite"),
	}
	_, err := c.Start(ctx, cfg)
	if err == nil {
		c.Stop(ctx)
		t.Error("Expected error when SQLite path is invalid, got nil")
	}
}

