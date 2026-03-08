package groupshare

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

// --- test helpers ---

type staticMemberResolver struct {
	members map[string][]string // groupID -> DIDs (already excluding self)
}

func (r *staticMemberResolver) MemberDIDsForGroup(_ context.Context, groupID string) ([]string, error) {
	m, ok := r.members[groupID]
	if !ok {
		return nil, errors.New("group not found")
	}
	return m, nil
}

type sendCapture struct {
	mu       sync.Mutex
	calls    []sendCall
}

type sendCall struct {
	DIDs    []string
	Payload []byte
}

func (c *sendCapture) send(_ context.Context, dids []string, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]byte, len(payload))
	copy(cp, payload)
	c.calls = append(c.calls, sendCall{DIDs: dids, Payload: cp})
	return nil
}

// allowAll is an AccessPolicy that allows everything.
type allowAll struct{}
func (allowAll) CanWrite(string) bool { return true }
func (allowAll) CanRead(string) bool  { return true }

// denyWrite is an AccessPolicy that denies all writes.
type denyWrite struct{}
func (denyWrite) CanWrite(string) bool { return false }
func (denyWrite) CanRead(string) bool  { return true }

// denyRead is an AccessPolicy that denies reads for a specific DID.
type denyRead struct{ blockedDID string }
func (denyRead) CanWrite(string) bool     { return true }
func (d denyRead) CanRead(did string) bool { return did != d.blockedDID }

// rejectSchema rejects all bodies.
type rejectSchema struct{}
func (rejectSchema) Validate([]byte) error { return errors.New("schema rejected") }

// --- MemSharedStorage tests ---

func TestMemSharedStorage_PutAndGet(t *testing.T) {
	ctx := context.Background()
	s := NewMemSharedStorage()

	rec := &SharedRecord{
		ID: "r1", Channel: "chat", GroupID: "g1",
		DID: "did:key:zAlice", Timestamp: 1000, Body: []byte(`hello`),
	}
	if err := s.PutShared(ctx, rec); err != nil {
		t.Fatalf("PutShared: %v", err)
	}

	got, err := s.GetShared(ctx, "chat", "r1")
	if err != nil {
		t.Fatalf("GetShared: %v", err)
	}
	if got == nil {
		t.Fatal("GetShared should return the stored record")
	}
	if string(got.Body) != `hello` {
		t.Fatalf("body = %q, want hello", got.Body)
	}
	if got.DID != "did:key:zAlice" {
		t.Fatalf("DID = %q", got.DID)
	}
}

func TestMemSharedStorage_GetNotFound(t *testing.T) {
	ctx := context.Background()
	s := NewMemSharedStorage()

	got, err := s.GetShared(ctx, "chat", "nope")
	if err != nil {
		t.Fatalf("GetShared: %v", err)
	}
	if got != nil {
		t.Fatal("should return nil for nonexistent")
	}
}

func TestMemSharedStorage_GetTimestamp(t *testing.T) {
	ctx := context.Background()
	s := NewMemSharedStorage()

	ts, _ := s.GetTimestamp(ctx, "chat", "r1")
	if ts != 0 {
		t.Fatalf("timestamp for nonexistent = %d, want 0", ts)
	}

	s.PutShared(ctx, &SharedRecord{ID: "r1", Channel: "chat", Timestamp: 1500})
	ts, _ = s.GetTimestamp(ctx, "chat", "r1")
	if ts != 1500 {
		t.Fatalf("timestamp = %d, want 1500", ts)
	}
}

func TestMemSharedStorage_Delete(t *testing.T) {
	ctx := context.Background()
	s := NewMemSharedStorage()

	s.PutShared(ctx, &SharedRecord{ID: "r1", Channel: "chat", Timestamp: 1000, Body: []byte(`x`)})
	s.DeleteShared(ctx, "chat", "r1")

	got, _ := s.GetShared(ctx, "chat", "r1")
	if got != nil {
		t.Fatal("should be nil after delete")
	}
}

func TestMemSharedStorage_ListByChannel(t *testing.T) {
	ctx := context.Background()
	s := NewMemSharedStorage()

	s.PutShared(ctx, &SharedRecord{ID: "r1", Channel: "chat", Body: []byte(`a`)})
	s.PutShared(ctx, &SharedRecord{ID: "r2", Channel: "chat", Body: []byte(`b`)})
	s.PutShared(ctx, &SharedRecord{ID: "r3", Channel: "files", Body: []byte(`c`)})

	recs, _ := s.ListByChannel(ctx, "chat")
	if len(recs) != 2 {
		t.Fatalf("ListByChannel(chat) = %d, want 2", len(recs))
	}

	recs2, _ := s.ListByChannel(ctx, "files")
	if len(recs2) != 1 {
		t.Fatalf("ListByChannel(files) = %d, want 1", len(recs2))
	}
}

// --- GroupShareLayer tests ---

func TestLayer_RegisterChannel(t *testing.T) {
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zSelf")

	err := layer.RegisterChannel(&Channel{Name: "chat", GroupID: "g1"})
	if err != nil {
		t.Fatalf("RegisterChannel: %v", err)
	}

	// Duplicate should fail
	err = layer.RegisterChannel(&Channel{Name: "chat", GroupID: "g1"})
	if err == nil {
		t.Fatal("duplicate RegisterChannel should fail")
	}
}

func TestLayer_PutAndBroadcast(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	capture := &sendCapture{}
	resolver := &staticMemberResolver{members: map[string][]string{
		"g1": {"did:key:zBob", "did:key:zCharlie"},
	}}

	layer := NewGroupShareLayer(storage, resolver, capture.send, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "chat", GroupID: "g1", Access: allowAll{}})

	err := layer.Put(ctx, "chat", "msg1", []byte(`hello`))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Should be stored locally
	rec, _ := storage.GetShared(ctx, "chat", "msg1")
	if rec == nil {
		t.Fatal("Put should store locally")
	}
	if string(rec.Body) != `hello` {
		t.Fatalf("body = %q", rec.Body)
	}
	if rec.DID != "did:key:zAlice" {
		t.Fatalf("DID = %q, want zAlice", rec.DID)
	}
	if rec.GroupID != "g1" {
		t.Fatalf("GroupID = %q, want g1", rec.GroupID)
	}

	// Should have sent to group members
	if len(capture.calls) != 1 {
		t.Fatalf("expected 1 send call, got %d", len(capture.calls))
	}
	if len(capture.calls[0].DIDs) != 2 {
		t.Fatalf("expected 2 DIDs, got %d", len(capture.calls[0].DIDs))
	}

	// Payload should be valid SharedRecord JSON
	var shared SharedRecord
	if err := json.Unmarshal(capture.calls[0].Payload, &shared); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if shared.Channel != "chat" || shared.ID != "msg1" {
		t.Fatalf("unexpected shared record: %+v", shared)
	}
}

func TestLayer_PutDeniedByAccessPolicy(t *testing.T) {
	ctx := context.Background()
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "chat", GroupID: "g1", Access: denyWrite{}})

	err := layer.Put(ctx, "chat", "msg1", []byte(`hello`))
	if err == nil {
		t.Fatal("Put should fail when CanWrite returns false")
	}
}

func TestLayer_PutUnregisteredChannel(t *testing.T) {
	ctx := context.Background()
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")

	err := layer.Put(ctx, "unknown", "msg1", []byte(`hello`))
	if err == nil {
		t.Fatal("Put to unregistered channel should fail")
	}
}

func TestLayer_Get(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	storage.PutShared(ctx, &SharedRecord{
		ID: "r1", Channel: "chat", Body: []byte(`stored`), DID: "did:key:zBob",
	})

	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")

	rec, err := layer.Get(ctx, "chat", "r1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil || string(rec.Body) != `stored` {
		t.Fatal("Get should return stored record")
	}
}

func TestLayer_Delete(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	capture := &sendCapture{}
	resolver := &staticMemberResolver{members: map[string][]string{"g1": {"did:key:zBob"}}}

	layer := NewGroupShareLayer(storage, resolver, capture.send, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "chat", GroupID: "g1", Access: allowAll{}})

	layer.Put(ctx, "chat", "msg1", []byte(`hello`))
	err := layer.Delete(ctx, "chat", "msg1")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	rec, _ := storage.GetShared(ctx, "chat", "msg1")
	if rec != nil {
		t.Fatal("Delete should remove record from storage")
	}

	// Should broadcast a delete SharedRecord
	if len(capture.calls) != 2 { // put + delete
		t.Fatalf("expected 2 send calls, got %d", len(capture.calls))
	}
	var shared SharedRecord
	json.Unmarshal(capture.calls[1].Payload, &shared)
	if !shared.Deleted {
		t.Fatal("delete broadcast should have Deleted=true")
	}
}

func TestLayer_List(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")

	storage.PutShared(ctx, &SharedRecord{ID: "r1", Channel: "chat", Body: []byte(`a`)})
	storage.PutShared(ctx, &SharedRecord{ID: "r2", Channel: "chat", Body: []byte(`b`)})

	recs, err := layer.List(ctx, "chat")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("List = %d, want 2", len(recs))
	}
}

func TestLayer_HandleIncoming_NewRecord(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "chat", GroupID: "g1", Access: allowAll{}})

	rec := &SharedRecord{
		ID: "msg1", Channel: "chat", GroupID: "g1",
		DID: "did:key:zBob", Timestamp: 1000, Body: []byte(`hi`),
	}
	payload, _ := json.Marshal(rec)

	err := layer.HandleIncoming(ctx, payload)
	if err != nil {
		t.Fatalf("HandleIncoming: %v", err)
	}

	got, _ := storage.GetShared(ctx, "chat", "msg1")
	if got == nil {
		t.Fatal("HandleIncoming should store new record")
	}
	if string(got.Body) != `hi` {
		t.Fatalf("body = %q", got.Body)
	}
}

func TestLayer_HandleIncoming_LastWriteWins(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "chat", GroupID: "g1", Access: allowAll{}})

	// Existing record with timestamp 2000
	storage.PutShared(ctx, &SharedRecord{
		ID: "msg1", Channel: "chat", Timestamp: 2000, Body: []byte(`current`),
	})

	// Older incoming should be skipped
	old, _ := json.Marshal(&SharedRecord{
		ID: "msg1", Channel: "chat", GroupID: "g1",
		DID: "did:key:zBob", Timestamp: 1000, Body: []byte(`old`),
	})
	layer.HandleIncoming(ctx, old)

	got, _ := storage.GetShared(ctx, "chat", "msg1")
	if string(got.Body) != `current` {
		t.Fatalf("older should be skipped, body = %q", got.Body)
	}

	// Newer incoming should be applied
	newer, _ := json.Marshal(&SharedRecord{
		ID: "msg1", Channel: "chat", GroupID: "g1",
		DID: "did:key:zBob", Timestamp: 3000, Body: []byte(`newer`),
	})
	layer.HandleIncoming(ctx, newer)

	got, _ = storage.GetShared(ctx, "chat", "msg1")
	if string(got.Body) != `newer` {
		t.Fatalf("newer should be applied, body = %q", got.Body)
	}
}

func TestLayer_HandleIncoming_Delete(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "chat", GroupID: "g1", Access: allowAll{}})

	storage.PutShared(ctx, &SharedRecord{
		ID: "msg1", Channel: "chat", Timestamp: 1000, Body: []byte(`x`),
	})

	del, _ := json.Marshal(&SharedRecord{
		ID: "msg1", Channel: "chat", GroupID: "g1",
		DID: "did:key:zBob", Timestamp: 2000, Deleted: true,
	})
	layer.HandleIncoming(ctx, del)

	got, _ := storage.GetShared(ctx, "chat", "msg1")
	if got != nil {
		t.Fatal("HandleIncoming Delete should remove record")
	}
}

func TestLayer_HandleIncoming_SchemaRejected(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{
		Name: "chat", GroupID: "g1",
		Schema: rejectSchema{}, Access: allowAll{},
	})

	rec, _ := json.Marshal(&SharedRecord{
		ID: "msg1", Channel: "chat", GroupID: "g1",
		DID: "did:key:zBob", Timestamp: 1000, Body: []byte(`bad`),
	})

	err := layer.HandleIncoming(ctx, rec)
	if err == nil {
		t.Fatal("HandleIncoming should fail when schema rejects body")
	}

	got, _ := storage.GetShared(ctx, "chat", "msg1")
	if got != nil {
		t.Fatal("rejected record should not be stored")
	}
}

func TestLayer_HandleIncoming_ReadDenied(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{
		Name: "chat", GroupID: "g1",
		Access: denyRead{blockedDID: "did:key:zBob"},
	})

	rec, _ := json.Marshal(&SharedRecord{
		ID: "msg1", Channel: "chat", GroupID: "g1",
		DID: "did:key:zBob", Timestamp: 1000, Body: []byte(`blocked`),
	})

	err := layer.HandleIncoming(ctx, rec)
	if err == nil {
		t.Fatal("HandleIncoming should fail when sender is blocked by AccessPolicy")
	}

	got, _ := storage.GetShared(ctx, "chat", "msg1")
	if got != nil {
		t.Fatal("blocked record should not be stored")
	}
}
