package syncdb

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSyncLayer_Put_AttachesMetaAndStores(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()
	var sentDIDs []string
	var sentPayload []byte
	send := func(ctx context.Context, memberDIDs []string, payload []byte) error {
		sentDIDs = memberDIDs
		sentPayload = payload
		return nil
	}
	resolverWithMembers := &mockResolver{members: []string{"did:a", "did:b"}}
	layer := NewSyncLayer(storage, resolverWithMembers, send, "did:self")

	record, err := layer.Put(ctx, "g1", []byte("hello"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if record.ID == "" {
		t.Error("Put should set ID")
	}
	if record.GroupID != "g1" {
		t.Errorf("GroupID: got %q", record.GroupID)
	}
	if record.DID != "did:self" {
		t.Errorf("DID: got %q", record.DID)
	}
	if record.Timestamp <= 0 {
		t.Error("Put should set Timestamp")
	}
	if string(record.Body) != "hello" {
		t.Errorf("Body: got %q", record.Body)
	}
	got, _ := storage.Get(ctx, record.ID)
	if got == nil {
		t.Fatal("record should be in storage")
	}
	if got.ID != record.ID || string(got.Body) != "hello" {
		t.Errorf("storage record: %+v", got)
	}
	if len(sentDIDs) != 2 {
		t.Errorf("SendGroup should be called with 2 members, got %v", sentDIDs)
	}
	var decoded SyncRecord
	if err := json.Unmarshal(sentPayload, &decoded); err != nil {
		t.Fatalf("sent payload should be JSON SyncRecord: %v", err)
	}
	if decoded.ID != record.ID || string(decoded.Body) != "hello" {
		t.Errorf("sent record: %+v", decoded)
	}
}

func TestSyncLayer_HandleIncoming_LastWriteWins_NewerApplied(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()
	layer := NewSyncLayer(storage, nil, nil, "did:self")

	// Apply first record
	r1 := &SyncRecord{ID: "id1", GroupID: "g1", DID: "did:a", Timestamp: 100, Body: []byte("v1")}
	if err := layer.HandleIncoming(ctx, mustMarshal(r1)); err != nil {
		t.Fatalf("HandleIncoming v1: %v", err)
	}
	got, _ := storage.Get(ctx, "id1")
	if got == nil || string(got.Body) != "v1" {
		t.Errorf("after v1: got %+v", got)
	}

	// Newer timestamp: apply
	r2 := &SyncRecord{ID: "id1", GroupID: "g1", DID: "did:b", Timestamp: 200, Body: []byte("v2")}
	if err := layer.HandleIncoming(ctx, mustMarshal(r2)); err != nil {
		t.Fatalf("HandleIncoming v2: %v", err)
	}
	got, _ = storage.Get(ctx, "id1")
	if got == nil || string(got.Body) != "v2" {
		t.Errorf("after v2: got %+v", got)
	}
}

func TestSyncLayer_HandleIncoming_LastWriteWins_OlderSkipped(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()
	layer := NewSyncLayer(storage, nil, nil, "did:self")

	r1 := &SyncRecord{ID: "id1", GroupID: "g1", DID: "did:a", Timestamp: 200, Body: []byte("v2")}
	if err := layer.HandleIncoming(ctx, mustMarshal(r1)); err != nil {
		t.Fatalf("HandleIncoming v2: %v", err)
	}
	r2 := &SyncRecord{ID: "id1", GroupID: "g1", DID: "did:b", Timestamp: 100, Body: []byte("v1")}
	if err := layer.HandleIncoming(ctx, mustMarshal(r2)); err != nil {
		t.Fatalf("HandleIncoming v1: %v", err)
	}
	got, _ := storage.Get(ctx, "id1")
	if got == nil || string(got.Body) != "v2" {
		t.Errorf("older should be skipped: got %+v", got)
	}
}

func TestSyncLayer_HandleIncoming_DeletedAppliesAsDelete(t *testing.T) {
	ctx := context.Background()
	storage := NewMemStorage()
	layer := NewSyncLayer(storage, nil, nil, "did:self")

	r1 := &SyncRecord{ID: "id1", GroupID: "g1", DID: "did:a", Timestamp: 100, Body: []byte("v1")}
	if err := layer.HandleIncoming(ctx, mustMarshal(r1)); err != nil {
		t.Fatalf("HandleIncoming: %v", err)
	}
	r2 := &SyncRecord{ID: "id1", GroupID: "g1", DID: "did:a", Timestamp: 200, Deleted: true}
	if err := layer.HandleIncoming(ctx, mustMarshal(r2)); err != nil {
		t.Fatalf("HandleIncoming delete: %v", err)
	}
	got, _ := storage.Get(ctx, "id1")
	if got != nil {
		t.Errorf("record should be deleted, got %+v", got)
	}
}

func mustMarshal(r *SyncRecord) []byte {
	b, _ := json.Marshal(r)
	return b
}

// mockResolver returns a fixed member list for tests.
type mockResolver struct {
	members []string
}

func (m *mockResolver) MemberDIDsForGroup(ctx context.Context, groupID string) ([]string, error) {
	return m.members, nil
}
