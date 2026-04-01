package network

import (
	"sync"

	"github.com/google/uuid"
)

// MemStore is an in-memory Store implementation for testing.
type MemStore struct {
	mu       sync.RWMutex
	networks map[string]*Network
}

// NewMemStore creates a new in-memory network store.
func NewMemStore() *MemStore {
	return &MemStore{networks: make(map[string]*Network)}
}

func (m *MemStore) CreateNetwork(n *Network) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	cp := *n
	cp.ID = id
	cp.MemberRoles = copyRoles(n.MemberRoles)
	cp.Members = copySlice(n.Members)
	m.networks[id] = &cp
	return id, nil
}

func (m *MemStore) GetNetwork(id string) (*Network, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	n, ok := m.networks[id]
	if !ok {
		return nil, nil
	}
	return n, nil
}

func (m *MemStore) UpdateNetwork(id string, n *Network) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.networks[id]; !ok {
		return ErrNetworkNotFound
	}
	cp := *n
	cp.ID = id
	cp.MemberRoles = copyRoles(n.MemberRoles)
	cp.Members = copySlice(n.Members)
	m.networks[id] = &cp
	return nil
}

func (m *MemStore) DeleteNetwork(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.networks, id)
	return nil
}

func (m *MemStore) ListForMember(memberDID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var ids []string
	for _, n := range m.networks {
		for _, m := range n.Members {
			if m == memberDID {
				ids = append(ids, n.ID)
				break
			}
		}
	}
	return ids, nil
}

func copySlice(s []string) []string {
	if s == nil {
		return nil
	}
	cp := make([]string, len(s))
	copy(cp, s)
	return cp
}

func copyRoles(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	cp := make(map[string]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}
