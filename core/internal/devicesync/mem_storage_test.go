package devicesync

import (
	"context"
	"testing"
)

func TestMemStorage_PutAndGet(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	seq, err := s.Put(ctx, "contacts", "alice", []byte(`{"name":"Alice"}`), 1000)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if seq == 0 {
		t.Fatal("Put should return a non-zero sequence number")
	}

	rec, err := s.Get(ctx, "contacts", "alice")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec == nil {
		t.Fatal("Get should return the stored record")
	}
	if string(rec.Body) != `{"name":"Alice"}` {
		t.Fatalf("Get body = %q, want %q", rec.Body, `{"name":"Alice"}`)
	}
	if rec.Timestamp != 1000 {
		t.Fatalf("Get timestamp = %d, want 1000", rec.Timestamp)
	}
}

func TestMemStorage_GetNotFound(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	rec, err := s.Get(ctx, "contacts", "nonexistent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec != nil {
		t.Fatal("Get should return nil for nonexistent record")
	}
}

func TestMemStorage_PutOverwrite(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	s.Put(ctx, "contacts", "alice", []byte(`{"name":"Alice"}`), 1000)
	seq2, _ := s.Put(ctx, "contacts", "alice", []byte(`{"name":"Alice Updated"}`), 2000)

	rec, _ := s.Get(ctx, "contacts", "alice")
	if string(rec.Body) != `{"name":"Alice Updated"}` {
		t.Fatalf("Get body = %q, want updated value", rec.Body)
	}
	if rec.Timestamp != 2000 {
		t.Fatalf("timestamp = %d, want 2000", rec.Timestamp)
	}
	if seq2 <= 1 {
		t.Fatal("second Put should have a higher sequence number")
	}
}

func TestMemStorage_Delete(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	s.Put(ctx, "contacts", "alice", []byte(`{"name":"Alice"}`), 1000)

	seq, err := s.Delete(ctx, "contacts", "alice", 2000)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if seq == 0 {
		t.Fatal("Delete should return a non-zero sequence number")
	}

	rec, _ := s.Get(ctx, "contacts", "alice")
	if rec != nil {
		t.Fatal("Get after Delete should return nil")
	}
}

func TestMemStorage_List(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	s.Put(ctx, "contacts", "alice", []byte(`a`), 1000)
	s.Put(ctx, "contacts", "bob", []byte(`b`), 1001)
	s.Put(ctx, "messages", "msg1", []byte(`m`), 1002)

	recs, err := s.List(ctx, "contacts")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("List contacts = %d records, want 2", len(recs))
	}

	recs2, _ := s.List(ctx, "messages")
	if len(recs2) != 1 {
		t.Fatalf("List messages = %d records, want 1", len(recs2))
	}

	recs3, _ := s.List(ctx, "empty")
	if len(recs3) != 0 {
		t.Fatalf("List empty = %d records, want 0", len(recs3))
	}
}

func TestMemStorage_GetTimestamp(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	ts, err := s.GetTimestamp(ctx, "contacts", "alice")
	if err != nil {
		t.Fatalf("GetTimestamp: %v", err)
	}
	if ts != 0 {
		t.Fatalf("GetTimestamp for nonexistent = %d, want 0", ts)
	}

	s.Put(ctx, "contacts", "alice", []byte(`a`), 1500)
	ts, _ = s.GetTimestamp(ctx, "contacts", "alice")
	if ts != 1500 {
		t.Fatalf("GetTimestamp = %d, want 1500", ts)
	}
}

func TestMemStorage_ChangeLog(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	// Initially empty
	seq, _ := s.LatestSeq(ctx)
	if seq != 0 {
		t.Fatalf("LatestSeq initially = %d, want 0", seq)
	}

	entries, _ := s.ChangesSince(ctx, 0)
	if len(entries) != 0 {
		t.Fatalf("ChangesSince(0) initially = %d entries, want 0", len(entries))
	}

	// Append entries
	e1 := &ChangeEntry{Seq: 1, Timestamp: 1000, Table: "t", RecordID: "a", Op: OpPut, Body: []byte("x")}
	e2 := &ChangeEntry{Seq: 2, Timestamp: 2000, Table: "t", RecordID: "b", Op: OpDelete}

	s.AppendChange(ctx, e1)
	s.AppendChange(ctx, e2)

	seq, _ = s.LatestSeq(ctx)
	if seq != 2 {
		t.Fatalf("LatestSeq = %d, want 2", seq)
	}

	// ChangesSince returns entries with Seq > since
	all, _ := s.ChangesSince(ctx, 0)
	if len(all) != 2 {
		t.Fatalf("ChangesSince(0) = %d entries, want 2", len(all))
	}

	after1, _ := s.ChangesSince(ctx, 1)
	if len(after1) != 1 {
		t.Fatalf("ChangesSince(1) = %d entries, want 1", len(after1))
	}
	if after1[0].Seq != 2 {
		t.Fatalf("ChangesSince(1)[0].Seq = %d, want 2", after1[0].Seq)
	}

	after2, _ := s.ChangesSince(ctx, 2)
	if len(after2) != 0 {
		t.Fatalf("ChangesSince(2) = %d entries, want 0", len(after2))
	}
}

func TestMemStorage_MinSeq_Empty(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	seq, err := s.MinSeq(ctx)
	if err != nil {
		t.Fatalf("MinSeq: %v", err)
	}
	if seq != 0 {
		t.Fatalf("MinSeq on empty log = %d, want 0", seq)
	}
}

func TestMemStorage_MinSeq_AfterAppend(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	s.Put(ctx, "t", "a", []byte("1"), 1000)
	s.Put(ctx, "t", "b", []byte("2"), 2000)
	s.Put(ctx, "t", "c", []byte("3"), 3000)

	seq, err := s.MinSeq(ctx)
	if err != nil {
		t.Fatalf("MinSeq: %v", err)
	}
	if seq != 1 {
		t.Fatalf("MinSeq = %d, want 1", seq)
	}
}

func TestMemStorage_TruncateChangeLog(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	s.Put(ctx, "t", "a", []byte("1"), 1000) // seq=1
	s.Put(ctx, "t", "b", []byte("2"), 2000) // seq=2
	s.Put(ctx, "t", "c", []byte("3"), 3000) // seq=3

	// Truncate entries with seq < 3 (removes seq=1 and seq=2)
	err := s.TruncateChangeLog(ctx, 3)
	if err != nil {
		t.Fatalf("TruncateChangeLog: %v", err)
	}

	min, _ := s.MinSeq(ctx)
	if min != 3 {
		t.Fatalf("MinSeq after truncate = %d, want 3", min)
	}
}

func TestMemStorage_TruncateChangeLog_ChangesSinceReflects(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	s.Put(ctx, "t", "a", []byte("1"), 1000) // seq=1
	s.Put(ctx, "t", "b", []byte("2"), 2000) // seq=2
	s.Put(ctx, "t", "c", []byte("3"), 3000) // seq=3
	s.Put(ctx, "t", "d", []byte("4"), 4000) // seq=4

	// Truncate entries with seq < 3
	s.TruncateChangeLog(ctx, 3)

	// ChangesSince(0) should only return seq=3 and seq=4
	entries, err := s.ChangesSince(ctx, 0)
	if err != nil {
		t.Fatalf("ChangesSince: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ChangesSince(0) after truncate = %d entries, want 2", len(entries))
	}
	if entries[0].Seq != 3 {
		t.Fatalf("first entry seq = %d, want 3", entries[0].Seq)
	}
	if entries[1].Seq != 4 {
		t.Fatalf("second entry seq = %d, want 4", entries[1].Seq)
	}

	// LatestSeq should be unchanged
	latest, _ := s.LatestSeq(ctx)
	if latest != 4 {
		t.Fatalf("LatestSeq = %d, want 4", latest)
	}
}

func TestMemStorage_SequenceIncrementsAcrossOps(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	seq1, _ := s.Put(ctx, "t", "a", []byte("1"), 1000)
	seq2, _ := s.Put(ctx, "t", "b", []byte("2"), 1001)
	seq3, _ := s.Delete(ctx, "t", "a", 1002)

	if seq1 >= seq2 || seq2 >= seq3 {
		t.Fatalf("sequences should be strictly increasing: %d, %d, %d", seq1, seq2, seq3)
	}
}
