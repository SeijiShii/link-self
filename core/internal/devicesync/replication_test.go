package devicesync

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// testSendCapture captures all payloads sent to peers.
type testSendCapture struct {
	mu       sync.Mutex
	payloads map[string][][]byte // peerID -> list of payloads
}

func newTestSendCapture() *testSendCapture {
	return &testSendCapture{payloads: make(map[string][][]byte)}
}

func (c *testSendCapture) send(ctx context.Context, peerID string, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]byte, len(payload))
	copy(cp, payload)
	c.payloads[peerID] = append(c.payloads[peerID], cp)
	return nil
}

func (c *testSendCapture) allPayloads() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	var all [][]byte
	for _, ps := range c.payloads {
		all = append(all, ps...)
	}
	return all
}

func TestReplicationEngine_PutBroadcasts(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()
	capture := newTestSendCapture()

	peers := func(ctx context.Context) ([]string, error) {
		return []string{"peer-phone", "peer-tablet"}, nil
	}

	engine := &ReplicationEngine{
		Storage: storage,
		Send:    capture.send,
		Peers:   peers,
		SelfDID: "did:key:zSelf",
	}

	err := engine.Put(ctx, "contacts", "alice", []byte(`{"name":"Alice"}`))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Should have been stored locally
	rec, _ := storage.Get(ctx, "contacts", "alice")
	if rec == nil {
		t.Fatal("Put should store record locally")
	}
	if string(rec.Body) != `{"name":"Alice"}` {
		t.Fatalf("stored body = %q", rec.Body)
	}

	// Should have sent to both peers
	for _, peerID := range []string{"peer-phone", "peer-tablet"} {
		if len(capture.payloads[peerID]) != 1 {
			t.Fatalf("expected 1 payload to %s, got %d", peerID, len(capture.payloads[peerID]))
		}
		// Payload should be a valid ChangeEntry JSON
		var entry ChangeEntry
		if err := json.Unmarshal(capture.payloads[peerID][0], &entry); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if entry.Table != "contacts" || entry.RecordID != "alice" || entry.Op != OpPut {
			t.Fatalf("unexpected entry: %+v", entry)
		}
	}

	// Change log should have the entry
	latestSeq, _ := storage.LatestSeq(ctx)
	if latestSeq == 0 {
		t.Fatal("change log should have an entry after Put")
	}
}

func TestReplicationEngine_Get(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()
	storage.Put(ctx, "contacts", "alice", []byte(`{"name":"Alice"}`), 1000)

	engine := &ReplicationEngine{Storage: storage}

	rec, err := engine.Get(ctx, "contacts", "alice")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil || string(rec.Body) != `{"name":"Alice"}` {
		t.Fatal("Get should return the stored record")
	}
}

func TestReplicationEngine_DeleteBroadcasts(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()
	capture := newTestSendCapture()

	peers := func(ctx context.Context) ([]string, error) {
		return []string{"peer1"}, nil
	}

	engine := &ReplicationEngine{
		Storage: storage,
		Send:    capture.send,
		Peers:   peers,
		SelfDID: "did:key:zSelf",
	}

	// Put first, then delete
	engine.Put(ctx, "contacts", "alice", []byte(`a`))
	err := engine.Delete(ctx, "contacts", "alice")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Should be gone locally
	rec, _ := storage.Get(ctx, "contacts", "alice")
	if rec != nil {
		t.Fatal("Delete should remove record locally")
	}

	// Should have sent delete to peer
	if len(capture.payloads["peer1"]) != 2 {
		t.Fatalf("expected 2 payloads (put + delete), got %d", len(capture.payloads["peer1"]))
	}
	var entry ChangeEntry
	json.Unmarshal(capture.payloads["peer1"][1], &entry)
	if entry.Op != OpDelete {
		t.Fatalf("second payload should be Delete, got %v", entry.Op)
	}
}

func TestReplicationEngine_List(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()
	engine := &ReplicationEngine{Storage: storage}

	storage.Put(ctx, "contacts", "a", []byte(`1`), 1000)
	storage.Put(ctx, "contacts", "b", []byte(`2`), 1001)

	recs, err := engine.List(ctx, "contacts")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("List = %d records, want 2", len(recs))
	}
}

func TestReplicationEngine_HandleIncoming_NewRecord(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()
	engine := &ReplicationEngine{Storage: storage}

	entry := &ChangeEntry{
		Seq:       1,
		Timestamp: 1000,
		Table:     "contacts",
		RecordID:  "alice",
		Op:        OpPut,
		Body:      []byte(`{"name":"Alice"}`),
	}

	err := engine.HandleIncoming(ctx, entry)
	if err != nil {
		t.Fatalf("HandleIncoming: %v", err)
	}

	rec, _ := storage.Get(ctx, "contacts", "alice")
	if rec == nil {
		t.Fatal("HandleIncoming should apply new record to storage")
	}
	if string(rec.Body) != `{"name":"Alice"}` {
		t.Fatalf("body = %q", rec.Body)
	}
}

func TestReplicationEngine_HandleIncoming_LastWriteWins(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()
	engine := &ReplicationEngine{Storage: storage}

	// Apply initial record
	storage.Put(ctx, "contacts", "alice", []byte(`old`), 2000)

	// Incoming with older timestamp should be skipped
	old := &ChangeEntry{
		Seq: 1, Timestamp: 1000, Table: "contacts", RecordID: "alice",
		Op: OpPut, Body: []byte(`ancient`),
	}
	engine.HandleIncoming(ctx, old)

	rec, _ := storage.Get(ctx, "contacts", "alice")
	if string(rec.Body) != `old` {
		t.Fatalf("older incoming should be skipped, body = %q", rec.Body)
	}

	// Incoming with newer timestamp should be applied
	newer := &ChangeEntry{
		Seq: 2, Timestamp: 3000, Table: "contacts", RecordID: "alice",
		Op: OpPut, Body: []byte(`newer`),
	}
	engine.HandleIncoming(ctx, newer)

	rec, _ = storage.Get(ctx, "contacts", "alice")
	if string(rec.Body) != `newer` {
		t.Fatalf("newer incoming should be applied, body = %q", rec.Body)
	}
}

func TestReplicationEngine_HandleIncoming_Delete(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()
	engine := &ReplicationEngine{Storage: storage}

	storage.Put(ctx, "contacts", "alice", []byte(`a`), 1000)

	del := &ChangeEntry{
		Seq: 1, Timestamp: 2000, Table: "contacts", RecordID: "alice",
		Op: OpDelete,
	}
	engine.HandleIncoming(ctx, del)

	rec, _ := storage.Get(ctx, "contacts", "alice")
	if rec != nil {
		t.Fatal("HandleIncoming Delete should remove record")
	}
}

func TestReplicationEngine_HandleIncoming_DeleteSkippedIfOlder(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()
	engine := &ReplicationEngine{Storage: storage}

	storage.Put(ctx, "contacts", "alice", []byte(`a`), 2000)

	del := &ChangeEntry{
		Seq: 1, Timestamp: 1000, Table: "contacts", RecordID: "alice",
		Op: OpDelete,
	}
	engine.HandleIncoming(ctx, del)

	rec, _ := storage.Get(ctx, "contacts", "alice")
	if rec == nil {
		t.Fatal("older Delete should be skipped")
	}
}

func TestReplicationEngine_Put_TriggersRetention_CountBased(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()

	engine := &ReplicationEngine{
		Storage: storage,
		SelfDID: "did:key:zSelf",
		Retention: &RetentionPolicy{
			TimeBased: false,
			MaxCount:  3,
		},
	}

	// Write 5 records — should retain only the latest 3
	for i := 0; i < 5; i++ {
		engine.Put(ctx, "t", string(rune('a'+i)), []byte("v"))
	}

	entries, _ := storage.ChangesSince(ctx, 0)
	if len(entries) > 3 {
		t.Fatalf("expected at most 3 entries after retention, got %d", len(entries))
	}

	min, _ := storage.MinSeq(ctx)
	if min < 3 {
		t.Fatalf("MinSeq should be >= 3 after truncation, got %d", min)
	}
}

func TestReplicationEngine_Put_TriggersRetention_TimeBased(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()

	// Manually insert old entries with timestamps far in the past
	oldTs := time.Now().Add(-48 * time.Hour).UnixMilli()
	storage.Put(ctx, "t", "old1", []byte("v"), oldTs)
	storage.Put(ctx, "t", "old2", []byte("v"), oldTs)

	engine := &ReplicationEngine{
		Storage: storage,
		SelfDID: "did:key:zSelf",
		Retention: &RetentionPolicy{
			TimeBased: true,
			MaxAge:    24 * time.Hour, // 1 day
		},
	}

	// Write a new record — should trigger retention and prune old entries
	engine.Put(ctx, "t", "new1", []byte("v"))

	entries, _ := storage.ChangesSince(ctx, 0)
	// Only the new entry should remain (old ones are > 24h ago)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after time-based retention, got %d", len(entries))
	}
}

func TestReplicationEngine_DefaultRetention(t *testing.T) {
	// Nil Retention should not panic and should not prune
	ctx := context.Background()
	storage := NewMemStorage()

	engine := &ReplicationEngine{
		Storage:   storage,
		SelfDID:   "did:key:zSelf",
		Retention: nil,
	}

	for i := 0; i < 10; i++ {
		engine.Put(ctx, "t", string(rune('a'+i)), []byte("v"))
	}

	entries, _ := storage.ChangesSince(ctx, 0)
	if len(entries) != 10 {
		t.Fatalf("nil retention should not prune, got %d entries", len(entries))
	}
}

func TestReplicationEngine_SyncWith_IncrementalSync(t *testing.T) {
	ctx := context.Background()

	// Device A has 3 entries
	storageA := NewMemStorage()
	engineA := &ReplicationEngine{Storage: storageA, SelfDID: "did:key:A"}
	engineA.Put(ctx, "t", "a", []byte("1"))
	engineA.Put(ctx, "t", "b", []byte("2"))
	engineA.Put(ctx, "t", "c", []byte("3"))

	// Device B has only the first entry (simulating partial sync)
	storageB := NewMemStorage()
	storageB.Put(ctx, "t", "a", []byte("1"), 1000)

	// B syncs with A: B sends its latest seq, A responds with missing entries
	var received []*ChangeEntry
	err := engineA.SyncWith(ctx, SyncPeer{
		LatestSeq: func(ctx context.Context) (uint64, error) {
			return storageB.LatestSeq(ctx)
		},
		SendEntries: func(ctx context.Context, entries []*ChangeEntry) error {
			received = entries
			return nil
		},
	})
	if err != nil {
		t.Fatalf("SyncWith: %v", err)
	}

	if len(received) != 2 {
		t.Fatalf("expected 2 entries sent, got %d", len(received))
	}
	if received[0].RecordID != "b" || received[1].RecordID != "c" {
		t.Fatalf("unexpected entries: %+v", received)
	}
}

func TestReplicationEngine_SyncWith_GapDetected_FullSync(t *testing.T) {
	ctx := context.Background()

	// Device A has entries but has truncated old ones
	storageA := NewMemStorage()
	engineA := &ReplicationEngine{
		Storage: storageA,
		SelfDID: "did:key:A",
		Retention: &RetentionPolicy{
			TimeBased: false,
			MaxCount:  2,
		},
	}
	engineA.Put(ctx, "t", "a", []byte("1")) // seq=1
	engineA.Put(ctx, "t", "b", []byte("2")) // seq=2
	engineA.Put(ctx, "t", "c", []byte("3")) // seq=3 → truncates seq=1

	// Verify truncation happened
	minA, _ := storageA.MinSeq(ctx)
	if minA < 2 {
		t.Fatalf("expected MinSeq >= 2 after truncation, got %d", minA)
	}

	// Device B is far behind (seq=0, never synced)
	var fullDump []*Record
	err := engineA.SyncWith(ctx, SyncPeer{
		LatestSeq: func(ctx context.Context) (uint64, error) {
			return 0, nil // B has nothing
		},
		SendEntries: func(ctx context.Context, entries []*ChangeEntry) error {
			t.Fatal("should not send incremental entries on gap")
			return nil
		},
		SendFullDump: func(ctx context.Context, records []*Record) error {
			fullDump = records
			return nil
		},
	})
	if err != nil {
		t.Fatalf("SyncWith: %v", err)
	}

	if len(fullDump) == 0 {
		t.Fatal("expected full dump on gap, got none")
	}
}

func TestReplicationEngine_SyncWith_AlreadyUpToDate(t *testing.T) {
	ctx := context.Background()

	storageA := NewMemStorage()
	engineA := &ReplicationEngine{Storage: storageA, SelfDID: "did:key:A"}
	engineA.Put(ctx, "t", "a", []byte("1"))

	latestA, _ := storageA.LatestSeq(ctx)

	var received []*ChangeEntry
	err := engineA.SyncWith(ctx, SyncPeer{
		LatestSeq: func(ctx context.Context) (uint64, error) {
			return latestA, nil // B is up to date
		},
		SendEntries: func(ctx context.Context, entries []*ChangeEntry) error {
			received = entries
			return nil
		},
	})
	if err != nil {
		t.Fatalf("SyncWith: %v", err)
	}

	if len(received) != 0 {
		t.Fatalf("expected 0 entries when up to date, got %d", len(received))
	}
}

func TestReplicationEngine_NoPeers(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()

	engine := &ReplicationEngine{
		Storage: storage,
		Send:    func(ctx context.Context, peerID string, payload []byte) error { t.Fatal("should not send"); return nil },
		Peers:   func(ctx context.Context) ([]string, error) { return nil, nil },
		SelfDID: "did:key:zSelf",
	}

	// Put should succeed even with no peers
	err := engine.Put(ctx, "contacts", "alice", []byte(`a`))
	if err != nil {
		t.Fatalf("Put with no peers: %v", err)
	}

	rec, _ := storage.Get(ctx, "contacts", "alice")
	if rec == nil {
		t.Fatal("Put should store locally even with no peers")
	}
}
