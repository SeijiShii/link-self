package group

import (
	"testing"
)

func TestMemStore_CreateGet(t *testing.T) {
	s := NewMemStore()
	members := []string{"did:a", "did:b"}
	owners := []string{"did:a"}
	id, err := s.CreateGroup(members, owners)
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if id == "" {
		t.Fatal("CreateGroup returned empty id")
	}
	g, err := s.GetGroup(id)
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	if g.ID != id {
		t.Errorf("GetGroup id %q, want %q", g.ID, id)
	}
	if len(g.Members) != 2 || g.Members[0] != "did:a" || g.Members[1] != "did:b" {
		t.Errorf("GetGroup members %v, want [did:a did:b]", g.Members)
	}
	if len(g.Owners) != 1 || g.Owners[0] != "did:a" {
		t.Errorf("GetGroup owners %v, want [did:a]", g.Owners)
	}
}

func TestMemStore_ListByMember(t *testing.T) {
	s := NewMemStore()
	m1 := []string{"did:a", "did:b"}
	m2 := []string{"did:a", "did:c"}
	id1, _ := s.CreateGroup(m1, []string{"did:a"})
	id2, _ := s.CreateGroup(m2, []string{"did:a"})
	ids, err := s.ListGroupIDsForMember("did:a")
	if err != nil {
		t.Fatalf("ListGroupIDsForMember: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("member did:a: got %d groups, want 2", len(ids))
	}
	seen := make(map[string]bool)
	for _, id := range ids {
		seen[id] = true
	}
	if !seen[id1] || !seen[id2] {
		t.Errorf("ListGroupIDsForMember did:a: got %v", ids)
	}
	idsB, _ := s.ListGroupIDsForMember("did:b")
	if len(idsB) != 1 || idsB[0] != id1 {
		t.Errorf("ListGroupIDsForMember did:b: got %v, want [%s]", idsB, id1)
	}
	idsNone, _ := s.ListGroupIDsForMember("did:x")
	if len(idsNone) != 0 {
		t.Errorf("ListGroupIDsForMember did:x: got %v, want []", idsNone)
	}
}

func TestMemStore_Update(t *testing.T) {
	s := NewMemStore()
	members := []string{"did:a", "did:b"}
	id, _ := s.CreateGroup(members, []string{"did:a"})
	newMembers := []string{"did:a", "did:b", "did:c"}
	newOwners := []string{"did:a", "did:b"}
	if err := s.UpdateGroup(id, newMembers, newOwners); err != nil {
		t.Fatalf("UpdateGroup: %v", err)
	}
	g, _ := s.GetGroup(id)
	if len(g.Members) != 3 || g.Members[2] != "did:c" {
		t.Errorf("after Update: members %v", g.Members)
	}
	if len(g.Owners) != 2 {
		t.Errorf("after Update: owners %v", g.Owners)
	}
}

func TestMemStore_Delete(t *testing.T) {
	s := NewMemStore()
	members := []string{"did:a", "did:b"}
	id, _ := s.CreateGroup(members, nil)
	if err := s.DeleteGroup(id); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	g, err := s.GetGroup(id)
	if err != nil {
		t.Fatalf("GetGroup after delete: %v", err)
	}
	if g != nil {
		t.Errorf("GetGroup after delete: got %+v, want nil", g)
	}
	ids, _ := s.ListGroupIDsForMember("did:a")
	if len(ids) != 0 {
		t.Errorf("ListGroupIDsForMember after delete: got %v", ids)
	}
}

func TestMemStore_GetNotFound(t *testing.T) {
	s := NewMemStore()
	g, err := s.GetGroup("nonexistent")
	if err != nil {
		t.Fatalf("GetGroup nonexistent: %v", err)
	}
	if g != nil {
		t.Errorf("GetGroup nonexistent: got %+v, want nil", g)
	}
}

func TestMemStore_CreateReturnsUniqueIDs(t *testing.T) {
	s := NewMemStore()
	members := []string{"did:a", "did:b"}
	id1, _ := s.CreateGroup(members, nil)
	id2, _ := s.CreateGroup(members, nil)
	if id1 == id2 {
		t.Errorf("CreateGroup returned same id twice: %q", id1)
	}
}
