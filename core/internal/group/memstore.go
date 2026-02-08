package group

import (
	"sync"

	"github.com/google/uuid"
)

// MemStore is an in-memory implementation of Store for tests and validation.
type MemStore struct {
	mu     sync.RWMutex
	byID   map[string]*Group
	byDid  map[string]map[string]struct{} // memberDID -> set of groupIDs
	nextID func() string
}

// NewMemStore returns a new in-memory Store with UUID-based group IDs.
func NewMemStore() *MemStore {
	return &MemStore{
		byID:   make(map[string]*Group),
		byDid:  make(map[string]map[string]struct{}),
		nextID: func() string { return uuid.New().String() },
	}
}

// CreateGroup saves a new group and returns its ID.
func (m *MemStore) CreateGroup(members []string, ownerDIDs []string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := m.nextID()
	g := &Group{
		ID:      id,
		Members: copySlice(members),
		Owners:  copySlice(ownerDIDs),
	}
	m.byID[id] = g
	for _, did := range members {
		if m.byDid[did] == nil {
			m.byDid[did] = make(map[string]struct{})
		}
		m.byDid[did][id] = struct{}{}
	}
	return id, nil
}

// GetGroup returns the group by ID, or nil if not found.
func (m *MemStore) GetGroup(groupID string) (*Group, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g, ok := m.byID[groupID]
	if !ok {
		return nil, nil
	}
	return copyGroup(g), nil
}

// ListGroupIDsForMember returns group IDs that include the given member DID.
func (m *MemStore) ListGroupIDsForMember(memberDID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	set, ok := m.byDid[memberDID]
	if !ok {
		return nil, nil
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	return ids, nil
}

// UpdateGroup updates members and owners for the group.
func (m *MemStore) UpdateGroup(groupID string, members []string, ownerDIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.byID[groupID]
	if !ok {
		return ErrGroupNotFound
	}
	oldMembers := g.Members
	g.Members = copySlice(members)
	g.Owners = copySlice(ownerDIDs)
	for _, did := range oldMembers {
		if set := m.byDid[did]; set != nil {
			delete(set, groupID)
			if len(set) == 0 {
				delete(m.byDid, did)
			}
		}
	}
	for _, did := range members {
		if m.byDid[did] == nil {
			m.byDid[did] = make(map[string]struct{})
		}
		m.byDid[did][groupID] = struct{}{}
	}
	return nil
}

// DeleteGroup removes the group.
func (m *MemStore) DeleteGroup(groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	g, ok := m.byID[groupID]
	if !ok {
		return ErrGroupNotFound
	}
	delete(m.byID, groupID)
	for _, did := range g.Members {
		if set := m.byDid[did]; set != nil {
			delete(set, groupID)
			if len(set) == 0 {
				delete(m.byDid, did)
			}
		}
	}
	return nil
}

func copySlice(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	return out
}

func copyGroup(g *Group) *Group {
	if g == nil {
		return nil
	}
	return &Group{
		ID:      g.ID,
		Members: copySlice(g.Members),
		Owners:  copySlice(g.Owners),
	}
}
