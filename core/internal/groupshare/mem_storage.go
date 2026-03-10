package groupshare

import (
	"context"
	"sync"
)

type sharedKey struct {
	channel string
	id      string
}

// MemSharedStorage is an in-memory implementation of SharedStorage for testing.
type MemSharedStorage struct {
	mu      sync.RWMutex
	records map[sharedKey]*SharedRecord
}

func NewMemSharedStorage() *MemSharedStorage {
	return &MemSharedStorage{
		records: make(map[sharedKey]*SharedRecord),
	}
}

func (m *MemSharedStorage) PutShared(_ context.Context, record *SharedRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := *record
	if record.Body != nil {
		cp.Body = make([]byte, len(record.Body))
		copy(cp.Body, record.Body)
	}
	m.records[sharedKey{record.Channel, record.ID}] = &cp
	return nil
}

func (m *MemSharedStorage) GetShared(_ context.Context, channel, id string) (*SharedRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rec, ok := m.records[sharedKey{channel, id}]
	if !ok {
		return nil, nil
	}
	return rec, nil
}

func (m *MemSharedStorage) GetTimestamp(_ context.Context, channel, id string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rec, ok := m.records[sharedKey{channel, id}]
	if !ok {
		return 0, nil
	}
	return rec.Timestamp, nil
}

func (m *MemSharedStorage) DeleteShared(_ context.Context, channel, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.records, sharedKey{channel, id})
	return nil
}

func (m *MemSharedStorage) ListByChannel(_ context.Context, channel string) ([]*SharedRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*SharedRecord
	for k, r := range m.records {
		if k.channel == channel {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *MemSharedStorage) ListByGroup(_ context.Context, groupID string) ([]*SharedRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*SharedRecord
	for _, r := range m.records {
		if r.GroupID == groupID {
			result = append(result, r)
		}
	}
	return result, nil
}
