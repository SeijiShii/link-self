package groupshare

import "testing"

func TestTopicMatches(t *testing.T) {
	tests := []struct {
		name       string
		subscribed []string
		topic      string
		want       bool
	}{
		{"exact match", []string{"001", "002"}, "001", true},
		{"wildcard matches any", []string{"*"}, "001", true},
		{"wildcard matches empty", []string{"*"}, "", true},
		{"no match", []string{"001", "002"}, "003", false},
		{"empty subscription = unsubscribed", []string{}, "001", false},
		{"nil subscription = no match", nil, "001", false},
		{"empty topic exact match", []string{""}, "", true},
		{"empty topic no match", []string{"001"}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TopicMatches(tt.subscribed, tt.topic)
			if got != tt.want {
				t.Errorf("TopicMatches(%v, %q) = %v, want %v", tt.subscribed, tt.topic, got, tt.want)
			}
		})
	}
}

func TestMemSubscriptionStore_SetAndGet(t *testing.T) {
	s := NewMemSubscriptionStore()

	// Initially nil
	topics, err := s.GetSubscription("did:key:zAlice", "docs")
	if err != nil {
		t.Fatal(err)
	}
	if topics != nil {
		t.Fatalf("expected nil for unset subscription, got %v", topics)
	}

	// Set subscription
	if err := s.SetSubscription("did:key:zAlice", "docs", []string{"001", "002"}); err != nil {
		t.Fatal(err)
	}

	topics, err = s.GetSubscription("did:key:zAlice", "docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) != 2 || topics[0] != "001" || topics[1] != "002" {
		t.Fatalf("got %v, want [001 002]", topics)
	}

	// Different DID should not see it
	topics2, _ := s.GetSubscription("did:key:zBob", "docs")
	if topics2 != nil {
		t.Fatalf("Bob should have nil, got %v", topics2)
	}

	// Different channel should not see it
	topics3, _ := s.GetSubscription("did:key:zAlice", "chat")
	if topics3 != nil {
		t.Fatalf("chat should have nil, got %v", topics3)
	}
}

func TestMemSubscriptionStore_Overwrite(t *testing.T) {
	s := NewMemSubscriptionStore()

	s.SetSubscription("did:key:zAlice", "docs", []string{"001"})
	s.SetSubscription("did:key:zAlice", "docs", []string{"002", "003"})

	topics, _ := s.GetSubscription("did:key:zAlice", "docs")
	if len(topics) != 2 || topics[0] != "002" {
		t.Fatalf("overwrite failed, got %v", topics)
	}
}

func TestMemSubscriptionStore_Unsubscribe(t *testing.T) {
	s := NewMemSubscriptionStore()

	s.SetSubscription("did:key:zAlice", "docs", []string{"001"})
	// Empty slice = unsubscribe
	s.SetSubscription("did:key:zAlice", "docs", []string{})

	topics, _ := s.GetSubscription("did:key:zAlice", "docs")
	if topics == nil {
		t.Fatal("empty slice should be stored as empty, not nil")
	}
	if len(topics) != 0 {
		t.Fatalf("expected empty, got %v", topics)
	}
}

func TestMemSubscriptionStore_GetAllSubscriptions(t *testing.T) {
	s := NewMemSubscriptionStore()

	s.SetSubscription("did:key:zAlice", "docs", []string{"001"})
	s.SetSubscription("did:key:zAlice", "chat", []string{"*"})
	s.SetSubscription("did:key:zBob", "docs", []string{"002"})

	all, err := s.GetAllSubscriptions("did:key:zAlice")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(all))
	}
	if len(all["docs"]) != 1 || all["docs"][0] != "001" {
		t.Fatalf("docs = %v", all["docs"])
	}
	if len(all["chat"]) != 1 || all["chat"][0] != "*" {
		t.Fatalf("chat = %v", all["chat"])
	}

	// Bob should only have docs
	allBob, _ := s.GetAllSubscriptions("did:key:zBob")
	if len(allBob) != 1 {
		t.Fatalf("Bob expected 1 channel, got %d", len(allBob))
	}

	// Unknown DID returns empty map
	allUnknown, _ := s.GetAllSubscriptions("did:key:zUnknown")
	if len(allUnknown) != 0 {
		t.Fatalf("unknown DID should return empty, got %v", allUnknown)
	}
}
