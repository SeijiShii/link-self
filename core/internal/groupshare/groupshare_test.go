package groupshare

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
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

	err := layer.Put(ctx, "chat", "", "msg1", []byte(`hello`))
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

	err := layer.Put(ctx, "chat", "", "msg1", []byte(`hello`))
	if err == nil {
		t.Fatal("Put should fail when CanWrite returns false")
	}
}

func TestLayer_PutUnregisteredChannel(t *testing.T) {
	ctx := context.Background()
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")

	err := layer.Put(ctx, "unknown", "", "msg1", []byte(`hello`))
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

	layer.Put(ctx, "chat", "", "msg1", []byte(`hello`))
	err := layer.Delete(ctx, "chat", "", "msg1")
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

// --- Subscription filtering tests ---

// perDIDSendCapture captures send calls per individual DID.
type perDIDSendCapture struct {
	mu    sync.Mutex
	calls []sendCall
}

func (c *perDIDSendCapture) send(_ context.Context, dids []string, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]byte, len(payload))
	copy(cp, payload)
	c.calls = append(c.calls, sendCall{DIDs: dids, Payload: cp})
	return nil
}

func (c *perDIDSendCapture) sentToDIDs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var all []string
	for _, call := range c.calls {
		all = append(all, call.DIDs...)
	}
	return all
}

func TestLayer_Put_FiltersBySubscription(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	capture := &perDIDSendCapture{}
	resolver := &staticMemberResolver{members: map[string][]string{
		"g1": {"did:key:zBob", "did:key:zCharlie", "did:key:zDave"},
	}}

	remoteSubs := NewMemSubscriptionStore()
	// Bob subscribes to topic "001" and "*" (wildcard)
	remoteSubs.SetSubscription("did:key:zBob", "docs", []string{"001", "*"})
	// Charlie subscribes to topic "002" only
	remoteSubs.SetSubscription("did:key:zCharlie", "docs", []string{"002"})
	// Dave has no subscription set (unknown) → should still receive (safe default)

	layer := NewGroupShareLayer(storage, resolver, capture.send, "did:key:zAlice")
	layer.RemoteSubs = remoteSubs
	layer.RegisterChannel(&Channel{Name: "docs", GroupID: "g1"})

	// Put topic "001" → Bob (subscribed to 001 & *) and Dave (unknown) should receive
	err := layer.Put(ctx, "docs", "001", "rec1", []byte(`data`))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	dids := capture.sentToDIDs()
	if len(dids) != 2 {
		t.Fatalf("expected 2 recipients, got %d: %v", len(dids), dids)
	}
	hasBob, hasDave := false, false
	for _, d := range dids {
		if d == "did:key:zBob" {
			hasBob = true
		}
		if d == "did:key:zDave" {
			hasDave = true
		}
		if d == "did:key:zCharlie" {
			t.Fatal("Charlie should NOT receive topic 001")
		}
	}
	if !hasBob {
		t.Fatal("Bob should receive topic 001")
	}
	if !hasDave {
		t.Fatal("Dave (unknown subscription) should receive by default")
	}
}

func TestLayer_Put_NoRemoteSubs_SendsToAll(t *testing.T) {
	ctx := context.Background()
	capture := &perDIDSendCapture{}
	resolver := &staticMemberResolver{members: map[string][]string{
		"g1": {"did:key:zBob", "did:key:zCharlie"},
	}}

	// No RemoteSubs set (nil) → backward compat, send to all
	layer := NewGroupShareLayer(NewMemSharedStorage(), resolver, capture.send, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "docs", GroupID: "g1"})

	layer.Put(ctx, "docs", "001", "rec1", []byte(`data`))

	dids := capture.sentToDIDs()
	if len(dids) != 2 {
		t.Fatalf("nil RemoteSubs should send to all, got %d", len(dids))
	}
}

func TestLayer_Put_EmptySubscription_Excluded(t *testing.T) {
	ctx := context.Background()
	capture := &perDIDSendCapture{}
	resolver := &staticMemberResolver{members: map[string][]string{
		"g1": {"did:key:zBob"},
	}}

	remoteSubs := NewMemSubscriptionStore()
	// Bob explicitly unsubscribed (empty slice)
	remoteSubs.SetSubscription("did:key:zBob", "docs", []string{})

	layer := NewGroupShareLayer(NewMemSharedStorage(), resolver, capture.send, "did:key:zAlice")
	layer.RemoteSubs = remoteSubs
	layer.RegisterChannel(&Channel{Name: "docs", GroupID: "g1"})

	layer.Put(ctx, "docs", "001", "rec1", []byte(`data`))

	dids := capture.sentToDIDs()
	if len(dids) != 0 {
		t.Fatalf("Bob (empty subscription) should be excluded, got %v", dids)
	}
}

func TestLayer_Delete_FiltersBySubscription(t *testing.T) {
	ctx := context.Background()
	capture := &perDIDSendCapture{}
	resolver := &staticMemberResolver{members: map[string][]string{
		"g1": {"did:key:zBob", "did:key:zCharlie"},
	}}

	remoteSubs := NewMemSubscriptionStore()
	remoteSubs.SetSubscription("did:key:zBob", "docs", []string{"001"})
	remoteSubs.SetSubscription("did:key:zCharlie", "docs", []string{"002"})

	layer := NewGroupShareLayer(NewMemSharedStorage(), resolver, capture.send, "did:key:zAlice")
	layer.RemoteSubs = remoteSubs
	layer.RegisterChannel(&Channel{Name: "docs", GroupID: "g1"})

	layer.Put(ctx, "docs", "001", "rec1", []byte(`data`))
	capture.calls = nil // reset

	layer.Delete(ctx, "docs", "001", "rec1")

	dids := capture.sentToDIDs()
	if len(dids) != 1 || dids[0] != "did:key:zBob" {
		t.Fatalf("Delete should filter same as Put, got %v", dids)
	}
}

func TestLayer_Subscribe(t *testing.T) {
	localSubs := NewMemSubscriptionStore()
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")
	layer.LocalSubs = localSubs
	layer.RegisterChannel(&Channel{Name: "docs", GroupID: "g1"})

	// Subscribe to registered channel
	if err := layer.Subscribe("docs", []string{"001"}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Verify stored in LocalSubs
	topics, _ := localSubs.GetSubscription("did:key:zAlice", "docs")
	if len(topics) != 1 || topics[0] != "001" {
		t.Fatalf("LocalSubs should store subscription, got %v", topics)
	}

	// Overwrite
	layer.Subscribe("docs", []string{"001", "002"})
	topics, _ = localSubs.GetSubscription("did:key:zAlice", "docs")
	if len(topics) != 2 {
		t.Fatalf("Subscribe overwrite failed, got %v", topics)
	}

	// Unsubscribe (empty)
	layer.Subscribe("docs", []string{})
	topics, _ = localSubs.GetSubscription("did:key:zAlice", "docs")
	if len(topics) != 0 {
		t.Fatalf("Unsubscribe failed, got %v", topics)
	}

	// Subscribe to unregistered channel should fail
	if err := layer.Subscribe("unknown", []string{"001"}); err == nil {
		t.Fatal("Subscribe to unregistered channel should fail")
	}
}

func TestLayer_Subscribe_NilLocalSubs(t *testing.T) {
	// LocalSubs が nil の場合でもエラーにならない（後方互換）
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "docs", GroupID: "g1"})

	if err := layer.Subscribe("docs", []string{"001"}); err != nil {
		t.Fatalf("Subscribe with nil LocalSubs should not error: %v", err)
	}
}

// --- Subscription announcement tests ---

func TestLayer_HandleSubAnnouncement(t *testing.T) {
	remoteSubs := NewMemSubscriptionStore()
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")
	layer.RemoteSubs = remoteSubs
	layer.RegisterChannel(&Channel{Name: "docs", GroupID: "g1"})

	ann := &SubAnnouncement{
		DID:     "did:key:zBob",
		Channel: "docs",
		Topics:  []string{"001", "002"},
	}
	payload, _ := json.Marshal(ann)

	err := layer.HandleSubAnnouncement("did:key:zBob", payload)
	if err != nil {
		t.Fatalf("HandleSubAnnouncement: %v", err)
	}

	topics, _ := remoteSubs.GetSubscription("did:key:zBob", "docs")
	if len(topics) != 2 || topics[0] != "001" {
		t.Fatalf("expected [001 002], got %v", topics)
	}
}

func TestLayer_HandleSubAnnouncement_DIDMismatch(t *testing.T) {
	remoteSubs := NewMemSubscriptionStore()
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")
	layer.RemoteSubs = remoteSubs
	layer.RegisterChannel(&Channel{Name: "docs", GroupID: "g1"})

	// Sender DID doesn't match announcement DID → reject (spoofing prevention)
	ann := &SubAnnouncement{
		DID:     "did:key:zCharlie",
		Channel: "docs",
		Topics:  []string{"001"},
	}
	payload, _ := json.Marshal(ann)

	err := layer.HandleSubAnnouncement("did:key:zBob", payload)
	if err == nil {
		t.Fatal("should reject DID mismatch")
	}

	topics, _ := remoteSubs.GetSubscription("did:key:zCharlie", "docs")
	if topics != nil {
		t.Fatal("spoofed subscription should not be stored")
	}
}

func TestLayer_HandleSubAnnouncement_NilRemoteSubs(t *testing.T) {
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "docs", GroupID: "g1"})

	ann := &SubAnnouncement{DID: "did:key:zBob", Channel: "docs", Topics: []string{"001"}}
	payload, _ := json.Marshal(ann)

	err := layer.HandleSubAnnouncement("did:key:zBob", payload)
	if err != nil {
		t.Fatalf("nil RemoteSubs should not error: %v", err)
	}
}

func TestLayer_BroadcastSubscription(t *testing.T) {
	capture := &perDIDSendCapture{}
	resolver := &staticMemberResolver{members: map[string][]string{
		"g1": {"did:key:zBob", "did:key:zCharlie"},
	}}

	layer := NewGroupShareLayer(NewMemSharedStorage(), resolver, nil, "did:key:zAlice")
	layer.LocalSubs = NewMemSubscriptionStore()
	layer.SendSubAnnounce = capture.send
	layer.RegisterChannel(&Channel{Name: "docs", GroupID: "g1"})

	err := layer.Subscribe("docs", []string{"001", "002"})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Verify broadcast happened
	dids := capture.sentToDIDs()
	if len(dids) != 2 {
		t.Fatalf("expected broadcast to 2 members, got %d", len(dids))
	}

	// Verify payload is SubAnnouncement
	var ann SubAnnouncement
	if err := json.Unmarshal(capture.calls[0].Payload, &ann); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ann.DID != "did:key:zAlice" || ann.Channel != "docs" || len(ann.Topics) != 2 {
		t.Fatalf("unexpected announcement: %+v", ann)
	}
}

// --- Dump / Restore tests ---

func TestMemSharedStorage_ListByGroup(t *testing.T) {
	ctx := context.Background()
	s := NewMemSharedStorage()

	s.PutShared(ctx, &SharedRecord{ID: "r1", Channel: "chat", GroupID: "g1", Body: []byte(`a`)})
	s.PutShared(ctx, &SharedRecord{ID: "r2", Channel: "docs", GroupID: "g1", Body: []byte(`b`)})
	s.PutShared(ctx, &SharedRecord{ID: "r3", Channel: "chat", GroupID: "g2", Body: []byte(`c`)})

	recs, err := s.ListByGroup(ctx, "g1")
	if err != nil {
		t.Fatalf("ListByGroup: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("ListByGroup(g1) = %d, want 2", len(recs))
	}

	recs2, _ := s.ListByGroup(ctx, "g2")
	if len(recs2) != 1 {
		t.Fatalf("ListByGroup(g2) = %d, want 1", len(recs2))
	}

	recs3, _ := s.ListByGroup(ctx, "nonexistent")
	if len(recs3) != 0 {
		t.Fatalf("ListByGroup(nonexistent) = %d, want 0", len(recs3))
	}
}

func TestLayer_Dump(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "chat", GroupID: "g1"})
	layer.RegisterChannel(&Channel{Name: "docs", GroupID: "g1"})
	layer.RegisterChannel(&Channel{Name: "other", GroupID: "g2"})

	// Put records in g1 channels
	storage.PutShared(ctx, &SharedRecord{ID: "r1", Channel: "chat", GroupID: "g1", Body: []byte(`a`), Timestamp: 1000})
	storage.PutShared(ctx, &SharedRecord{ID: "r2", Channel: "docs", GroupID: "g1", Body: []byte(`b`), Timestamp: 2000})
	// Put record in g2 channel (should not appear in g1 dump)
	storage.PutShared(ctx, &SharedRecord{ID: "r3", Channel: "other", GroupID: "g2", Body: []byte(`c`), Timestamp: 3000})

	data, err := layer.Dump(ctx, "g1")
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}

	// Should contain only g1 records
	if len(data) != 2 {
		t.Fatalf("Dump(g1) = %d records, want 2", len(data))
	}

	// Verify JSON round-trip
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Marshal dump: %v", err)
	}
	var restored []*SharedRecord
	if err := json.Unmarshal(jsonBytes, &restored); err != nil {
		t.Fatalf("Unmarshal dump: %v", err)
	}
	if len(restored) != 2 {
		t.Fatalf("round-trip = %d records, want 2", len(restored))
	}
}

func TestLayer_Dump_EmptyGroup(t *testing.T) {
	ctx := context.Background()
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")

	data, err := layer.Dump(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Dump empty: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("Dump empty = %d, want 0", len(data))
	}
}

func TestLayer_Restore_NewRecords(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "chat", GroupID: "g1"})

	records := []*SharedRecord{
		{ID: "r1", Channel: "chat", GroupID: "g1", DID: "did:key:zBob", Body: []byte(`hello`), Timestamp: 1000},
		{ID: "r2", Channel: "chat", GroupID: "g1", DID: "did:key:zBob", Body: []byte(`world`), Timestamp: 2000},
	}

	restored, err := layer.Restore(ctx, records)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if restored != 2 {
		t.Fatalf("Restore = %d, want 2", restored)
	}

	// Verify records exist in storage
	r1, _ := storage.GetShared(ctx, "chat", "r1")
	if r1 == nil || string(r1.Body) != `hello` {
		t.Fatal("r1 should be restored")
	}
	r2, _ := storage.GetShared(ctx, "chat", "r2")
	if r2 == nil || string(r2.Body) != `world` {
		t.Fatal("r2 should be restored")
	}
}

func TestLayer_Restore_LWW(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "chat", GroupID: "g1"})

	// Existing record with timestamp 2000
	storage.PutShared(ctx, &SharedRecord{
		ID: "r1", Channel: "chat", GroupID: "g1",
		DID: "did:key:zAlice", Body: []byte(`current`), Timestamp: 2000,
	})

	records := []*SharedRecord{
		// Older → should be skipped
		{ID: "r1", Channel: "chat", GroupID: "g1", DID: "did:key:zBob", Body: []byte(`old`), Timestamp: 1000},
		// Newer → should overwrite
		{ID: "r1", Channel: "chat", GroupID: "g1", DID: "did:key:zBob", Body: []byte(`newer`), Timestamp: 3000},
	}

	restored, err := layer.Restore(ctx, records)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	// Only the newer record should be applied
	if restored != 1 {
		t.Fatalf("Restore = %d, want 1 (only newer applied)", restored)
	}

	got, _ := storage.GetShared(ctx, "chat", "r1")
	if string(got.Body) != `newer` {
		t.Fatalf("LWW failed: body = %q, want newer", got.Body)
	}
}

func TestLayer_Restore_Empty(t *testing.T) {
	ctx := context.Background()
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")

	restored, err := layer.Restore(ctx, nil)
	if err != nil {
		t.Fatalf("Restore nil: %v", err)
	}
	if restored != 0 {
		t.Fatalf("Restore nil = %d, want 0", restored)
	}

	restored, err = layer.Restore(ctx, []*SharedRecord{})
	if err != nil {
		t.Fatalf("Restore empty: %v", err)
	}
	if restored != 0 {
		t.Fatalf("Restore empty = %d, want 0", restored)
	}
}

// --- Retention / TTL tests ---

func TestIsExpired_PermanentChannel(t *testing.T) {
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")
	// Retention=0 (default) = permanent
	layer.RegisterChannel(&Channel{Name: "profiles", GroupID: "g1"})

	rec := &SharedRecord{ID: "r1", Channel: "profiles", Timestamp: 1000}
	// Even with a very large "now", permanent records never expire
	if layer.IsExpired(rec, 999999999999) {
		t.Fatal("permanent channel records should never expire")
	}
}

func TestIsExpired_WithRetention(t *testing.T) {
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{
		Name: "chat", GroupID: "g1",
		Retention: 1000 * time.Millisecond, // 1000ms retention
	})

	rec := &SharedRecord{ID: "r1", Channel: "chat", Timestamp: 5000}

	// now=5500 → within retention (5000+1000=6000) → not expired
	if layer.IsExpired(rec, 5500) {
		t.Fatal("record within retention should not be expired")
	}

	// now=6000 → at boundary → expired (>=)
	if !layer.IsExpired(rec, 6000) {
		t.Fatal("record at retention boundary should be expired")
	}

	// now=7000 → past retention → expired
	if !layer.IsExpired(rec, 7000) {
		t.Fatal("record past retention should be expired")
	}
}

func TestIsExpired_UnknownChannel(t *testing.T) {
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")

	rec := &SharedRecord{ID: "r1", Channel: "unknown", Timestamp: 1000}
	if layer.IsExpired(rec, 999999999999) {
		t.Fatal("unknown channel records should not be considered expired")
	}
}

func TestGet_FiltersExpired(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{
		Name: "chat", GroupID: "g1",
		Retention: 60 * time.Second,
	})

	// Record with old timestamp (expired)
	storage.PutShared(ctx, &SharedRecord{
		ID: "r1", Channel: "chat", GroupID: "g1",
		Body: []byte(`old`), Timestamp: 1000,
	})

	got, err := layer.Get(ctx, "chat", "r1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != nil {
		t.Fatal("Get should return nil for expired record")
	}
}

func TestGet_ReturnsNonExpired(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "profiles", GroupID: "g1"}) // permanent

	storage.PutShared(ctx, &SharedRecord{
		ID: "r1", Channel: "profiles", GroupID: "g1",
		Body: []byte(`data`), Timestamp: 1000,
	})

	got, err := layer.Get(ctx, "profiles", "r1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || string(got.Body) != `data` {
		t.Fatal("Get should return non-expired record")
	}
}

func TestList_FiltersExpired(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{
		Name: "chat", GroupID: "g1",
		Retention: 60 * time.Second,
	})

	// Old (expired) record
	storage.PutShared(ctx, &SharedRecord{
		ID: "r1", Channel: "chat", GroupID: "g1",
		Body: []byte(`old`), Timestamp: 1000,
	})
	// Very recent record (not expired — timestamp close to now)
	now := time.Now().UnixMilli()
	storage.PutShared(ctx, &SharedRecord{
		ID: "r2", Channel: "chat", GroupID: "g1",
		Body: []byte(`new`), Timestamp: now,
	})

	recs, err := layer.List(ctx, "chat")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("List should return 1 non-expired record, got %d", len(recs))
	}
	if recs[0].ID != "r2" {
		t.Fatalf("List should return r2, got %s", recs[0].ID)
	}
}

func TestDump_FiltersExpired(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{
		Name: "chat", GroupID: "g1",
		Retention: 60 * time.Second,
	})
	layer.RegisterChannel(&Channel{Name: "profiles", GroupID: "g1"}) // permanent

	// Expired chat record
	storage.PutShared(ctx, &SharedRecord{
		ID: "r1", Channel: "chat", GroupID: "g1",
		Body: []byte(`old`), Timestamp: 1000,
	})
	// Permanent profile record (old timestamp but no retention)
	storage.PutShared(ctx, &SharedRecord{
		ID: "r2", Channel: "profiles", GroupID: "g1",
		Body: []byte(`profile`), Timestamp: 1000,
	})

	data, err := layer.Dump(ctx, "g1")
	if err != nil {
		t.Fatalf("Dump: %v", err)
	}
	// Only the permanent record should remain
	if len(data) != 1 {
		t.Fatalf("Dump should return 1 non-expired record, got %d", len(data))
	}
	if data[0].ID != "r2" {
		t.Fatalf("Dump should return r2, got %s", data[0].ID)
	}
}

func TestHandleIncoming_RejectsExpired(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{
		Name: "chat", GroupID: "g1",
		Retention: 60 * time.Second, Access: allowAll{},
	})

	// Incoming record with very old timestamp (expired on arrival)
	rec, _ := json.Marshal(&SharedRecord{
		ID: "msg1", Channel: "chat", GroupID: "g1",
		DID: "did:key:zBob", Timestamp: 1000, Body: []byte(`old`),
	})

	err := layer.HandleIncoming(ctx, rec)
	if err != nil {
		t.Fatalf("HandleIncoming expired should not error: %v", err)
	}

	got, _ := storage.GetShared(ctx, "chat", "msg1")
	if got != nil {
		t.Fatal("expired incoming record should not be stored")
	}
}

func TestPurge_DeletesExpired(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{
		Name: "chat", GroupID: "g1",
		Retention: 60 * time.Second,
	})

	// Old (expired) record
	storage.PutShared(ctx, &SharedRecord{
		ID: "r1", Channel: "chat", GroupID: "g1",
		Body: []byte(`old`), Timestamp: 1000,
	})
	// Recent record
	now := time.Now().UnixMilli()
	storage.PutShared(ctx, &SharedRecord{
		ID: "r2", Channel: "chat", GroupID: "g1",
		Body: []byte(`new`), Timestamp: now,
	})

	purged, err := layer.Purge(ctx, "chat")
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if purged != 1 {
		t.Fatalf("Purge = %d, want 1", purged)
	}

	// r1 should be gone from storage
	r1, _ := storage.GetShared(ctx, "chat", "r1")
	if r1 != nil {
		t.Fatal("purged record should be deleted from storage")
	}
	// r2 should remain
	r2, _ := storage.GetShared(ctx, "chat", "r2")
	if r2 == nil {
		t.Fatal("non-expired record should remain after purge")
	}
}

func TestPurge_PermanentChannel(t *testing.T) {
	ctx := context.Background()
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	layer.RegisterChannel(&Channel{Name: "profiles", GroupID: "g1"}) // Retention=0

	storage.PutShared(ctx, &SharedRecord{
		ID: "r1", Channel: "profiles", GroupID: "g1",
		Body: []byte(`data`), Timestamp: 1000,
	})

	purged, err := layer.Purge(ctx, "profiles")
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if purged != 0 {
		t.Fatalf("Purge on permanent channel = %d, want 0", purged)
	}
}

func TestPurge_UnregisteredChannel(t *testing.T) {
	ctx := context.Background()
	layer := NewGroupShareLayer(NewMemSharedStorage(), nil, nil, "did:key:zAlice")

	_, err := layer.Purge(ctx, "unknown")
	if err == nil {
		t.Fatal("Purge on unregistered channel should fail")
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

func TestLayer_AnnounceAllSubscriptions(t *testing.T) {
	storage := NewMemSharedStorage()
	resolver := &staticMemberResolver{
		members: map[string][]string{
			"g1": {"did:key:zBob"},
			"g2": {"did:key:zBob"},
		},
	}
	capture := &sendCapture{}
	layer := NewGroupShareLayer(storage, resolver, capture.send, "did:key:zAlice")
	layer.LocalSubs = NewMemSubscriptionStore()
	layer.RemoteSubs = NewMemSubscriptionStore()
	layer.SendSubAnnounce = capture.send

	// Register channels and subscribe
	layer.RegisterChannel(&Channel{Name: "feed", GroupID: "g1"})
	layer.RegisterChannel(&Channel{Name: "docs", GroupID: "g2"})
	layer.Subscribe("feed", []string{"sports", "news"})
	layer.Subscribe("docs", []string{"*"})

	// Clear captures from Subscribe broadcasts
	capture.mu.Lock()
	capture.calls = nil
	capture.mu.Unlock()

	// AnnounceAllSubscriptions should re-broadcast all subscriptions
	err := layer.AnnounceAllSubscriptions(context.Background())
	if err != nil {
		t.Fatalf("AnnounceAllSubscriptions: %v", err)
	}

	capture.mu.Lock()
	defer capture.mu.Unlock()
	if len(capture.calls) != 2 {
		t.Fatalf("expected 2 sub announcements, got %d", len(capture.calls))
	}

	// Verify announcements contain correct data
	for _, call := range capture.calls {
		var ann SubAnnouncement
		if err := json.Unmarshal(call.Payload, &ann); err != nil {
			t.Fatalf("unmarshal announcement: %v", err)
		}
		if ann.DID != "did:key:zAlice" {
			t.Errorf("announcement DID = %q, want did:key:zAlice", ann.DID)
		}
	}
}

func TestLayer_AnnounceAllSubscriptions_NilLocalSubs(t *testing.T) {
	storage := NewMemSharedStorage()
	layer := NewGroupShareLayer(storage, nil, nil, "did:key:zAlice")
	// LocalSubs is nil — should not panic
	err := layer.AnnounceAllSubscriptions(context.Background())
	if err != nil {
		t.Fatalf("AnnounceAllSubscriptions with nil LocalSubs: %v", err)
	}
}
