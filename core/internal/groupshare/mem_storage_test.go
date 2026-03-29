package groupshare

import (
	"context"
	"testing"
)

func TestMemSharedStorage_ListByChannelAndTopic(t *testing.T) {
	ctx := context.Background()
	s := NewMemSharedStorage()

	// Seed records with different topics
	records := []*SharedRecord{
		{ID: "1", Channel: "ch1", Topic: "news", GroupID: "g1", DID: "did:a", Timestamp: 100, Body: []byte("a")},
		{ID: "2", Channel: "ch1", Topic: "alert", GroupID: "g1", DID: "did:a", Timestamp: 200, Body: []byte("b")},
		{ID: "3", Channel: "ch1", Topic: "news", GroupID: "g1", DID: "did:b", Timestamp: 300, Body: []byte("c")},
		{ID: "4", Channel: "ch2", Topic: "news", GroupID: "g1", DID: "did:a", Timestamp: 400, Body: []byte("d")},
	}
	for _, r := range records {
		if err := s.PutShared(ctx, r); err != nil {
			t.Fatalf("PutShared: %v", err)
		}
	}

	// Filter ch1 + news → should return records 1,3
	got, err := s.ListByChannelAndTopic(ctx, "ch1", "news")
	if err != nil {
		t.Fatalf("ListByChannelAndTopic: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 records, got %d", len(got))
	}
	ids := map[string]bool{}
	for _, r := range got {
		ids[r.ID] = true
	}
	if !ids["1"] || !ids["3"] {
		t.Errorf("expected IDs 1,3 got %v", ids)
	}

	// Filter ch1 + alert → should return record 2
	got, err = s.ListByChannelAndTopic(ctx, "ch1", "alert")
	if err != nil {
		t.Fatalf("ListByChannelAndTopic: %v", err)
	}
	if len(got) != 1 || got[0].ID != "2" {
		t.Errorf("expected [2], got %v", got)
	}

	// Filter non-existent topic → empty
	got, err = s.ListByChannelAndTopic(ctx, "ch1", "missing")
	if err != nil {
		t.Fatalf("ListByChannelAndTopic: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}

func TestMemSharedStorage_DeleteExpired(t *testing.T) {
	ctx := context.Background()
	s := NewMemSharedStorage()

	// Seed: timestamps 100, 200, 300, 400
	records := []*SharedRecord{
		{ID: "1", Channel: "ch1", Topic: "t", GroupID: "g1", DID: "did:a", Timestamp: 100, Body: []byte("a")},
		{ID: "2", Channel: "ch1", Topic: "t", GroupID: "g1", DID: "did:a", Timestamp: 200, Body: []byte("b")},
		{ID: "3", Channel: "ch1", Topic: "t", GroupID: "g1", DID: "did:a", Timestamp: 300, Body: []byte("c")},
		{ID: "4", Channel: "ch1", Topic: "t", GroupID: "g1", DID: "did:a", Timestamp: 400, Body: []byte("d")},
		{ID: "5", Channel: "ch2", Topic: "t", GroupID: "g1", DID: "did:a", Timestamp: 100, Body: []byte("e")},
	}
	for _, r := range records {
		if err := s.PutShared(ctx, r); err != nil {
			t.Fatalf("PutShared: %v", err)
		}
	}

	// Delete ch1 records with timestamp < 250 → should delete IDs 1,2
	deleted, err := s.DeleteExpired(ctx, "ch1", 250)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	// Verify remaining ch1 records
	remaining, err := s.ListByChannel(ctx, "ch1")
	if err != nil {
		t.Fatalf("ListByChannel: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining in ch1, got %d", len(remaining))
	}

	// ch2 should be untouched
	ch2, err := s.ListByChannel(ctx, "ch2")
	if err != nil {
		t.Fatalf("ListByChannel ch2: %v", err)
	}
	if len(ch2) != 1 {
		t.Errorf("expected 1 in ch2, got %d", len(ch2))
	}

	// Delete with before=0 → nothing deleted
	deleted, err = s.DeleteExpired(ctx, "ch1", 0)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", deleted)
	}
}
