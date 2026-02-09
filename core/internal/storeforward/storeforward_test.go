package storeforward

import (
	"errors"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestQueue_PendingCount(t *testing.T) {
	s := New()
	if s.PendingCount("did:key:z6Mk") != 0 {
		t.Error("new store should have 0 pending")
	}
	s.Queue("did:key:z6Mk", []byte("a"))
	s.Queue("did:key:z6Mk", []byte("b"))
	if s.PendingCount("did:key:z6Mk") != 2 {
		t.Errorf("expected 2 pending, got %d", s.PendingCount("did:key:z6Mk"))
	}
	if s.PendingCount("other") != 0 {
		t.Error("other DID should have 0 pending")
	}
}

func TestFlushForDID_AllSent(t *testing.T) {
	s := New()
	pid := peer.ID("")
	s.Queue("did:key:z6Mk", []byte("msg1"))
	s.Queue("did:key:z6Mk", []byte("msg2"))
	var sent int
	sendFn := func(_ peer.ID, payload []byte) error {
		sent++
		_ = payload
		return nil
	}
	n, err := s.FlushForDID("did:key:z6Mk", pid, sendFn)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || sent != 2 {
		t.Errorf("expected 2 sent, got n=%d sent=%d", n, sent)
	}
	if s.PendingCount("did:key:z6Mk") != 0 {
		t.Error("queue should be empty after flush")
	}
}

func TestFlushForDID_PartialFailureRequeues(t *testing.T) {
	s := New()
	pid := peer.ID("")
	s.Queue("did:key:z6Mk", []byte("msg1"))
	s.Queue("did:key:z6Mk", []byte("msg2"))
	callCount := 0
	sendFn := func(_ peer.ID, payload []byte) error {
		callCount++
		if callCount == 2 {
			return errors.New("send failed")
		}
		return nil
	}
	n, err := s.FlushForDID("did:key:z6Mk", pid, sendFn)
	if err == nil {
		t.Fatal("expected error")
	}
	if n != 1 {
		t.Errorf("expected 1 sent before failure, got %d", n)
	}
	if s.PendingCount("did:key:z6Mk") != 1 {
		t.Errorf("remaining message should be requeued, got pending %d", s.PendingCount("did:key:z6Mk"))
	}
}
