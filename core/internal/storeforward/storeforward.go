// Package storeforward implements Store-and-Forward: queue messages by destination DID,
// flush when the peer is reported online (e.g. after auth).
package storeforward

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// SendFunc sends a single message payload to the given peer. Called when flushing the queue.
type SendFunc func(pid peer.ID, payload []byte) error

// StoreForward holds pending messages per DID and flushes when a peer is online.
type StoreForward struct {
	mu      sync.Mutex
	pending map[string][][]byte
}

// New returns a new StoreForward.
func New() *StoreForward {
	return &StoreForward{
		pending: make(map[string][][]byte),
	}
}

// Queue adds a message for the given DID. Call when the peer is offline or unknown.
func (s *StoreForward) Queue(did string, payload []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending[did] == nil {
		s.pending[did] = [][]byte{}
	}
	s.pending[did] = append(s.pending[did], payload)
}

// FlushForDID sends all queued messages for the given DID to the given peer using sendFn.
// Clears the queue for this DID. Call when the peer is known to be online (e.g. after auth).
func (s *StoreForward) FlushForDID(did string, pid peer.ID, sendFn SendFunc) (sent int, err error) {
	s.mu.Lock()
	msgs := s.pending[did]
	s.pending[did] = nil
	s.mu.Unlock()
	for _, payload := range msgs {
		if sendErr := sendFn(pid, payload); sendErr != nil {
			// Re-queue remaining on first send failure
			s.mu.Lock()
			s.pending[did] = append(msgs[sent:], s.pending[did]...)
			s.mu.Unlock()
			return sent, sendErr
		}
		sent++
	}
	return sent, nil
}

// PendingCount returns the number of queued messages for the DID (or 0).
func (s *StoreForward) PendingCount(did string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending[did])
}
