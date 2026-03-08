package devicesync

import (
	"context"
	"sync"
)

// recordKey uniquely identifies a record by table and id.
type recordKey struct {
	table string
	id    string
}

// MemStorage is an in-memory implementation of DeviceStorage for testing.
type MemStorage struct {
	mu      sync.RWMutex
	records map[recordKey]*Record
	log     []*ChangeEntry
	seq     uint64
}

// NewMemStorage creates a new in-memory DeviceStorage.
func NewMemStorage() *MemStorage {
	return &MemStorage{
		records: make(map[recordKey]*Record),
	}
}

func (m *MemStorage) nextSeq() uint64 {
	m.seq++
	return m.seq
}

func (m *MemStorage) Put(_ context.Context, table, id string, body []byte, timestamp int64) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	seq := m.nextSeq()
	cp := make([]byte, len(body))
	copy(cp, body)

	m.records[recordKey{table, id}] = &Record{
		ID:        id,
		Table:     table,
		Body:      cp,
		Timestamp: timestamp,
	}

	m.log = append(m.log, &ChangeEntry{
		Seq:       seq,
		Timestamp: timestamp,
		Table:     table,
		RecordID:  id,
		Op:        OpPut,
		Body:      cp,
	})

	return seq, nil
}

func (m *MemStorage) Get(_ context.Context, table, id string) (*Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rec, ok := m.records[recordKey{table, id}]
	if !ok {
		return nil, nil
	}
	return rec, nil
}

func (m *MemStorage) Delete(_ context.Context, table, id string, timestamp int64) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	seq := m.nextSeq()
	delete(m.records, recordKey{table, id})

	m.log = append(m.log, &ChangeEntry{
		Seq:       seq,
		Timestamp: timestamp,
		Table:     table,
		RecordID:  id,
		Op:        OpDelete,
	})

	return seq, nil
}

func (m *MemStorage) List(_ context.Context, table string) ([]*Record, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Record
	for k, r := range m.records {
		if k.table == table {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *MemStorage) GetTimestamp(_ context.Context, table, id string) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rec, ok := m.records[recordKey{table, id}]
	if !ok {
		return 0, nil
	}
	return rec.Timestamp, nil
}

func (m *MemStorage) AppendChange(_ context.Context, entry *ChangeEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.log = append(m.log, entry)
	if entry.Seq > m.seq {
		m.seq = entry.Seq
	}
	return nil
}

func (m *MemStorage) ChangesSince(_ context.Context, since uint64) ([]*ChangeEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ChangeEntry
	for _, e := range m.log {
		if e.Seq > since {
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *MemStorage) LatestSeq(_ context.Context) (uint64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.seq, nil
}
