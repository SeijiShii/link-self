package syncdb

import (
	"context"
	"sync"
)

// MemStorage is an in-memory RecordStorage for tests and validation.
type MemStorage struct {
	mu   sync.RWMutex
	byID map[string]*SyncRecord
}

// NewMemStorage returns a new in-memory RecordStorage.
func NewMemStorage() *MemStorage {
	return &MemStorage{byID: make(map[string]*SyncRecord)}
}

// Put stores the record (copy).
func (m *MemStorage) Put(ctx context.Context, record *SyncRecord) error {
	if record == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[record.ID] = copyRecord(record)
	return nil
}

// Get returns the record by ID, or nil if not found.
func (m *MemStorage) Get(ctx context.Context, id string) (*SyncRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.byID[id]
	if !ok {
		return nil, nil
	}
	return copyRecord(r), nil
}

// GetTimestamp returns the record's timestamp, or 0 if not found (apply allowed).
func (m *MemStorage) GetTimestamp(ctx context.Context, id string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.byID[id]
	if !ok {
		return 0, nil
	}
	return r.Timestamp, nil
}

// Delete removes the record.
func (m *MemStorage) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.byID, id)
	return nil
}

func copyRecord(r *SyncRecord) *SyncRecord {
	if r == nil {
		return nil
	}
	body := make([]byte, len(r.Body))
	copy(body, r.Body)
	return &SyncRecord{
		ID:        r.ID,
		GroupID:   r.GroupID,
		DID:       r.DID,
		Timestamp: r.Timestamp,
		Body:      body,
		Deleted:   r.Deleted,
	}
}
