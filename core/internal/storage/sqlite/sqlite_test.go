package sqlite

import (
	"context"
	"sort"
	"testing"

	"github.com/SeijiShii/link-self/core/internal/devicesync"
	"github.com/SeijiShii/link-self/core/internal/group"
	"github.com/SeijiShii/link-self/core/internal/groupshare"
	"github.com/SeijiShii/link-self/core/internal/syncdb"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// --- RecordStorage ---

func TestRecordStorage_PutGetDelete(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).RecordStorage()

	// Get non-existent
	r, err := s.Get(ctx, "nope")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if r != nil {
		t.Fatal("expected nil for missing record")
	}

	// Put
	rec := &syncdb.SyncRecord{
		ID: "r1", GroupID: "g1", DID: "did:a", Timestamp: 100, Body: []byte("hello"),
	}
	if err := s.Put(ctx, rec); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Get
	got, err := s.Get(ctx, "r1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.ID != "r1" || string(got.Body) != "hello" {
		t.Fatalf("unexpected: %+v", got)
	}

	// GetTimestamp
	ts, err := s.GetTimestamp(ctx, "r1")
	if err != nil {
		t.Fatalf("GetTimestamp: %v", err)
	}
	if ts != 100 {
		t.Fatalf("timestamp = %d, want 100", ts)
	}

	// GetTimestamp missing
	ts, err = s.GetTimestamp(ctx, "nope")
	if err != nil {
		t.Fatalf("GetTimestamp missing: %v", err)
	}
	if ts != 0 {
		t.Fatalf("timestamp missing = %d, want 0", ts)
	}

	// Delete
	if err := s.Delete(ctx, "r1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, _ = s.Get(ctx, "r1")
	if got != nil {
		t.Fatal("expected nil after delete")
	}

	// Put nil is no-op
	if err := s.Put(ctx, nil); err != nil {
		t.Fatalf("Put nil: %v", err)
	}
}

func TestRecordStorage_List(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).RecordStorage()

	// Empty
	got, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}

	// Add 3 records
	for _, r := range []*syncdb.SyncRecord{
		{ID: "r1", GroupID: "g1", DID: "did:a", Timestamp: 100, Body: []byte("a")},
		{ID: "r2", GroupID: "g1", DID: "did:b", Timestamp: 200, Body: []byte("b")},
		{ID: "r3", GroupID: "g2", DID: "did:a", Timestamp: 300, Body: []byte("c")},
	} {
		if err := s.Put(ctx, r); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	got, err = s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
}

func TestRecordStorage_PutOverwrite(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).RecordStorage()

	s.Put(ctx, &syncdb.SyncRecord{ID: "r1", GroupID: "g1", DID: "did:a", Timestamp: 100, Body: []byte("v1")})
	s.Put(ctx, &syncdb.SyncRecord{ID: "r1", GroupID: "g1", DID: "did:a", Timestamp: 200, Body: []byte("v2")})

	got, _ := s.Get(ctx, "r1")
	if string(got.Body) != "v2" {
		t.Fatalf("expected v2, got %s", got.Body)
	}
}

// --- SharedStorage ---

func TestSharedStorage_CRUD(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).SharedStorage()

	// Put
	rec := &groupshare.SharedRecord{
		ID: "s1", Channel: "ch1", Topic: "news", GroupID: "g1",
		DID: "did:a", Timestamp: 100, Body: []byte("data"),
	}
	if err := s.PutShared(ctx, rec); err != nil {
		t.Fatalf("PutShared: %v", err)
	}

	// Get
	got, err := s.GetShared(ctx, "ch1", "s1")
	if err != nil {
		t.Fatalf("GetShared: %v", err)
	}
	if got == nil || got.ID != "s1" || got.Topic != "news" {
		t.Fatalf("unexpected: %+v", got)
	}

	// Get missing
	got, err = s.GetShared(ctx, "ch1", "nope")
	if err != nil {
		t.Fatalf("GetShared missing: %v", err)
	}
	if got != nil {
		t.Fatal("expected nil")
	}

	// GetTimestamp
	ts, _ := s.GetTimestamp(ctx, "ch1", "s1")
	if ts != 100 {
		t.Fatalf("timestamp = %d", ts)
	}

	// Delete
	if err := s.DeleteShared(ctx, "ch1", "s1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, _ = s.GetShared(ctx, "ch1", "s1")
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestSharedStorage_ListByChannel(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).SharedStorage()

	for _, r := range []*groupshare.SharedRecord{
		{ID: "1", Channel: "ch1", GroupID: "g1", DID: "did:a", Timestamp: 100},
		{ID: "2", Channel: "ch1", GroupID: "g1", DID: "did:a", Timestamp: 200},
		{ID: "3", Channel: "ch2", GroupID: "g1", DID: "did:a", Timestamp: 300},
	} {
		s.PutShared(ctx, r)
	}

	got, err := s.ListByChannel(ctx, "ch1")
	if err != nil {
		t.Fatalf("ListByChannel: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestSharedStorage_ListByGroup(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).SharedStorage()

	for _, r := range []*groupshare.SharedRecord{
		{ID: "1", Channel: "ch1", GroupID: "g1", DID: "did:a", Timestamp: 100},
		{ID: "2", Channel: "ch2", GroupID: "g1", DID: "did:a", Timestamp: 200},
		{ID: "3", Channel: "ch1", GroupID: "g2", DID: "did:a", Timestamp: 300},
	} {
		s.PutShared(ctx, r)
	}

	got, err := s.ListByGroup(ctx, "g1")
	if err != nil {
		t.Fatalf("ListByGroup: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestSharedStorage_ListByChannelAndTopic(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).SharedStorage()

	for _, r := range []*groupshare.SharedRecord{
		{ID: "1", Channel: "ch1", Topic: "news", GroupID: "g1", DID: "did:a", Timestamp: 100},
		{ID: "2", Channel: "ch1", Topic: "alert", GroupID: "g1", DID: "did:a", Timestamp: 200},
		{ID: "3", Channel: "ch1", Topic: "news", GroupID: "g1", DID: "did:b", Timestamp: 300},
	} {
		s.PutShared(ctx, r)
	}

	got, err := s.ListByChannelAndTopic(ctx, "ch1", "news")
	if err != nil {
		t.Fatalf("ListByChannelAndTopic: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestSharedStorage_DeleteExpired(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).SharedStorage()

	for _, r := range []*groupshare.SharedRecord{
		{ID: "1", Channel: "ch1", GroupID: "g1", DID: "did:a", Timestamp: 100},
		{ID: "2", Channel: "ch1", GroupID: "g1", DID: "did:a", Timestamp: 200},
		{ID: "3", Channel: "ch1", GroupID: "g1", DID: "did:a", Timestamp: 300},
	} {
		s.PutShared(ctx, r)
	}

	n, err := s.DeleteExpired(ctx, "ch1", 250)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if n != 2 {
		t.Fatalf("deleted = %d, want 2", n)
	}

	remaining, _ := s.ListByChannel(ctx, "ch1")
	if len(remaining) != 1 {
		t.Fatalf("remaining = %d, want 1", len(remaining))
	}
}

// --- DeviceStorage ---

func TestDeviceStorage_PutGetDeleteList(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).DeviceStorage()

	// Get missing
	r, err := s.Get(ctx, "t1", "nope")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if r != nil {
		t.Fatal("expected nil")
	}

	// Put
	seq, err := s.Put(ctx, "t1", "id1", []byte("data"), 100)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if seq == 0 {
		t.Fatal("expected seq > 0")
	}

	// Get
	got, err := s.Get(ctx, "t1", "id1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || string(got.Body) != "data" {
		t.Fatalf("unexpected: %+v", got)
	}

	// GetTimestamp
	ts, _ := s.GetTimestamp(ctx, "t1", "id1")
	if ts != 100 {
		t.Fatalf("ts = %d", ts)
	}

	// List
	s.Put(ctx, "t1", "id2", []byte("data2"), 200)
	s.Put(ctx, "t2", "id3", []byte("data3"), 300)
	list, err := s.List(ctx, "t1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2, got %d", len(list))
	}

	// Delete
	delSeq, err := s.Delete(ctx, "t1", "id1", 400)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if delSeq <= seq {
		t.Fatalf("delete seq %d should be > put seq %d", delSeq, seq)
	}
	got, _ = s.Get(ctx, "t1", "id1")
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestDeviceStorage_ChangeLog(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).DeviceStorage()

	// LatestSeq empty
	seq, err := s.LatestSeq(ctx)
	if err != nil {
		t.Fatalf("LatestSeq: %v", err)
	}
	if seq != 0 {
		t.Fatalf("expected 0, got %d", seq)
	}

	// Put generates change entries
	s.Put(ctx, "t1", "id1", []byte("a"), 100)
	s.Put(ctx, "t1", "id2", []byte("b"), 200)
	s.Delete(ctx, "t1", "id1", 300)

	latest, _ := s.LatestSeq(ctx)
	if latest < 3 {
		t.Fatalf("expected >= 3, got %d", latest)
	}

	// ChangesSince
	changes, err := s.ChangesSince(ctx, 0)
	if err != nil {
		t.Fatalf("ChangesSince: %v", err)
	}
	if len(changes) < 3 {
		t.Fatalf("expected >= 3, got %d", len(changes))
	}

	// Verify ordering
	for i := 1; i < len(changes); i++ {
		if changes[i].Seq <= changes[i-1].Seq {
			t.Fatal("changes not ordered by seq")
		}
	}

	// ChangesSince with offset
	changes2, _ := s.ChangesSince(ctx, changes[0].Seq)
	if len(changes2) != len(changes)-1 {
		t.Fatalf("expected %d, got %d", len(changes)-1, len(changes2))
	}
}

func TestDeviceStorage_AppendChange(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).DeviceStorage()

	err := s.AppendChange(ctx, &devicesync.ChangeEntry{
		Seq: 999, Timestamp: 100, Table: "t1", RecordID: "id1",
		Op: devicesync.OpPut, Body: []byte("ext"),
	})
	if err != nil {
		t.Fatalf("AppendChange: %v", err)
	}

	latest, _ := s.LatestSeq(ctx)
	if latest != 999 {
		t.Fatalf("expected 999, got %d", latest)
	}

	changes, _ := s.ChangesSince(ctx, 0)
	if len(changes) != 1 || string(changes[0].Body) != "ext" {
		t.Fatalf("unexpected: %+v", changes)
	}
}

// --- SubscriptionStore ---

func TestSubscriptionStore_SetGetAll(t *testing.T) {
	db := openTestDB(t)
	s := db.SubscriptionStore()

	// Get missing
	topics, err := s.GetSubscription("did:a", "ch1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if topics != nil {
		t.Fatal("expected nil")
	}

	// Set
	if err := s.SetSubscription("did:a", "ch1", []string{"news", "alert"}); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := s.SetSubscription("did:a", "ch2", []string{"*"}); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Get
	topics, err = s.GetSubscription("did:a", "ch1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(topics) != 2 || topics[0] != "news" {
		t.Fatalf("unexpected: %v", topics)
	}

	// GetAll
	all, err := s.GetAllSubscriptions("did:a")
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(all))
	}

	// Overwrite
	s.SetSubscription("did:a", "ch1", []string{"only-news"})
	topics, _ = s.GetSubscription("did:a", "ch1")
	if len(topics) != 1 || topics[0] != "only-news" {
		t.Fatalf("overwrite failed: %v", topics)
	}
}

// --- GroupStore ---

func TestGroupStore_CRUD(t *testing.T) {
	db := openTestDB(t)
	s := db.GroupStore()

	// Create
	id, err := s.CreateGroup([]string{"did:a", "did:b"}, []string{"did:a"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	// Get
	g, err := s.GetGroup(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if g == nil {
		t.Fatal("expected group")
	}
	sort.Strings(g.Members)
	if len(g.Members) != 2 || g.Members[0] != "did:a" {
		t.Fatalf("members: %v", g.Members)
	}
	if len(g.Owners) != 1 || g.Owners[0] != "did:a" {
		t.Fatalf("owners: %v", g.Owners)
	}

	// Get missing
	g, err = s.GetGroup("nope")
	if err != nil {
		t.Fatalf("Get missing: %v", err)
	}
	if g != nil {
		t.Fatal("expected nil")
	}

	// ListGroupIDsForMember
	ids, err := s.ListGroupIDsForMember("did:b")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 1 || ids[0] != id {
		t.Fatalf("ids: %v", ids)
	}

	// ListGroupIDsForMember not a member
	ids, _ = s.ListGroupIDsForMember("did:c")
	if len(ids) != 0 {
		t.Fatalf("expected 0, got %d", len(ids))
	}

	// Update
	err = s.UpdateGroup(id, []string{"did:a", "did:c"}, []string{"did:a", "did:c"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	g, _ = s.GetGroup(id)
	sort.Strings(g.Members)
	if len(g.Members) != 2 || g.Members[1] != "did:c" {
		t.Fatalf("updated members: %v", g.Members)
	}

	// Update not found
	err = s.UpdateGroup("nope", nil, nil)
	if err != group.ErrGroupNotFound {
		t.Fatalf("expected ErrGroupNotFound, got %v", err)
	}

	// Delete
	err = s.DeleteGroup(id)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	g, _ = s.GetGroup(id)
	if g != nil {
		t.Fatal("expected nil after delete")
	}

	// Delete not found
	err = s.DeleteGroup(id)
	if err != group.ErrGroupNotFound {
		t.Fatalf("expected ErrGroupNotFound, got %v", err)
	}
}

// --- Open error ---

func TestOpen_InvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/dir/db.sqlite")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

// --- SharedStorage: Deleted flag round-trip ---

func TestSharedStorage_DeletedFlag(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).SharedStorage()

	rec := &groupshare.SharedRecord{
		ID: "d1", Channel: "ch1", GroupID: "g1", DID: "did:a",
		Timestamp: 100, Deleted: true,
	}
	s.PutShared(ctx, rec)
	got, _ := s.GetShared(ctx, "ch1", "d1")
	if got == nil || !got.Deleted {
		t.Fatal("Deleted flag not preserved")
	}
}

// --- RecordStorage: Deleted flag ---

func TestRecordStorage_DeletedFlag(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).RecordStorage()

	rec := &syncdb.SyncRecord{
		ID: "d1", GroupID: "g1", DID: "did:a", Timestamp: 100, Deleted: true,
	}
	s.Put(ctx, rec)
	got, _ := s.Get(ctx, "d1")
	if got == nil || !got.Deleted {
		t.Fatal("Deleted flag not preserved")
	}
}

// --- DeviceStorage: GetTimestamp missing ---

func TestDeviceStorage_GetTimestampMissing(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).DeviceStorage()
	ts, err := s.GetTimestamp(ctx, "t1", "nope")
	if err != nil {
		t.Fatalf("GetTimestamp: %v", err)
	}
	if ts != 0 {
		t.Fatalf("expected 0, got %d", ts)
	}
}

// --- SubscriptionStore: GetAllSubscriptions empty ---

func TestSubscriptionStore_GetAllEmpty(t *testing.T) {
	s := openTestDB(t).SubscriptionStore()
	all, err := s.GetAllSubscriptions("did:nobody")
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected empty, got %d", len(all))
	}
}

// --- SharedStorage: ListByChannel empty ---

func TestSharedStorage_ListByChannelEmpty(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).SharedStorage()
	got, err := s.ListByChannel(ctx, "empty")
	if err != nil {
		t.Fatalf("ListByChannel: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0, got %d", len(got))
	}
}

// --- SharedStorage: GetTimestamp missing ---

func TestSharedStorage_GetTimestampMissing(t *testing.T) {
	ctx := context.Background()
	s := openTestDB(t).SharedStorage()
	ts, err := s.GetTimestamp(ctx, "ch1", "nope")
	if err != nil {
		t.Fatalf("GetTimestamp: %v", err)
	}
	if ts != 0 {
		t.Fatalf("expected 0, got %d", ts)
	}
}

// --- Migration ---

func TestMigration_Idempotent(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Second migration should be no-op
	if err := db.migrate(); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}

	var v int
	db.conn.QueryRow("PRAGMA user_version").Scan(&v)
	if v != schemaVersion {
		t.Fatalf("version = %d, want %d", v, schemaVersion)
	}
}
