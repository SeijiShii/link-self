package syncdb

import (
	"context"
	"testing"
)

func TestMemStorage_List(t *testing.T) {
	ctx := context.Background()
	s := NewMemStorage()

	// Empty list
	got, err := s.List(ctx)
	if err != nil {
		t.Fatalf("List empty: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 records, got %d", len(got))
	}

	// Add records
	records := []*SyncRecord{
		{ID: "r1", GroupID: "g1", DID: "did:a", Timestamp: 100, Body: []byte("a")},
		{ID: "r2", GroupID: "g1", DID: "did:b", Timestamp: 200, Body: []byte("b")},
		{ID: "r3", GroupID: "g2", DID: "did:a", Timestamp: 300, Body: []byte("c")},
	}
	for _, r := range records {
		if err := s.Put(ctx, r); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}

	got, err = s.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 records, got %d", len(got))
	}

	// Verify records are copies (mutation safety)
	got[0].Body[0] = 'X'
	original, _ := s.Get(ctx, got[0].ID)
	if original.Body[0] == 'X' {
		t.Error("List should return copies, not references")
	}
}
