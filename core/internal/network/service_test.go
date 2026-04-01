package network

import (
	"testing"

	"github.com/SeijiShii/link-self/core/internal/role"
)

func makeDAG(t *testing.T) *role.DAG {
	t.Helper()
	dag, err := role.NewDAG(role.RoleDefs{
		"viewer": {},
		"editor": {Includes: []string{"viewer"}},
		"admin":  {Includes: []string{"editor"}},
	})
	if err != nil {
		t.Fatalf("NewDAG: %v", err)
	}
	return dag
}

func TestCreate_SingleMember(t *testing.T) {
	s := NewService(NewMemStore(), makeDAG(t), "admin")
	id, err := s.Create("suite-1", "did:key:alice")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Fatal("Create returned empty ID")
	}

	n, _ := s.store.GetNetwork(id)
	if len(n.Members) != 1 || n.Members[0] != "did:key:alice" {
		t.Errorf("Members = %v, want [did:key:alice]", n.Members)
	}
}

func TestCreate_SuiteID(t *testing.T) {
	s := NewService(NewMemStore(), makeDAG(t), "admin")
	id, _ := s.Create("my-suite", "did:key:alice")

	n, _ := s.store.GetNetwork(id)
	if n.SuiteID != "my-suite" {
		t.Errorf("SuiteID = %q, want my-suite", n.SuiteID)
	}
}

func TestAddMember_AdminCanAdd(t *testing.T) {
	s := NewService(NewMemStore(), makeDAG(t), "admin")
	id, _ := s.Create("s", "did:key:alice")

	// Alice is creator, gets admin role
	err := s.AddMember(id, "did:key:alice", "did:key:bob", "viewer")
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	n, _ := s.store.GetNetwork(id)
	if len(n.Members) != 2 {
		t.Errorf("Members count = %d, want 2", len(n.Members))
	}
}

func TestAddMember_NonAdminCannotAdd(t *testing.T) {
	s := NewService(NewMemStore(), makeDAG(t), "admin")
	id, _ := s.Create("s", "did:key:alice")

	// Add bob as viewer
	_ = s.AddMember(id, "did:key:alice", "did:key:bob", "viewer")

	// Bob (viewer) tries to add carol — should fail
	err := s.AddMember(id, "did:key:bob", "did:key:carol", "viewer")
	if err != ErrNoPermission {
		t.Errorf("expected ErrNoPermission, got %v", err)
	}
}

func TestAddMember_AlreadyMember(t *testing.T) {
	s := NewService(NewMemStore(), makeDAG(t), "admin")
	id, _ := s.Create("s", "did:key:alice")

	err := s.AddMember(id, "did:key:alice", "did:key:alice", "admin")
	if err != ErrAlreadyMember {
		t.Errorf("expected ErrAlreadyMember, got %v", err)
	}
}

func TestLeave(t *testing.T) {
	s := NewService(NewMemStore(), makeDAG(t), "admin")
	id, _ := s.Create("s", "did:key:alice")
	_ = s.AddMember(id, "did:key:alice", "did:key:bob", "viewer")

	err := s.Leave(id, "did:key:bob")
	if err != nil {
		t.Fatalf("Leave: %v", err)
	}

	n, _ := s.store.GetNetwork(id)
	if len(n.Members) != 1 {
		t.Errorf("Members = %d after leave, want 1", len(n.Members))
	}
}

func TestLeave_LastMember_DeletesNetwork(t *testing.T) {
	s := NewService(NewMemStore(), makeDAG(t), "admin")
	id, _ := s.Create("s", "did:key:alice")

	err := s.Leave(id, "did:key:alice")
	if err != nil {
		t.Fatalf("Leave: %v", err)
	}

	n, _ := s.store.GetNetwork(id)
	if n != nil {
		t.Error("network should be deleted when last member leaves")
	}
}

func TestLeave_NotMember(t *testing.T) {
	s := NewService(NewMemStore(), makeDAG(t), "admin")
	id, _ := s.Create("s", "did:key:alice")

	err := s.Leave(id, "did:key:nobody")
	if err != ErrNotMember {
		t.Errorf("expected ErrNotMember, got %v", err)
	}
}

func TestKick_AdminCanKick(t *testing.T) {
	s := NewService(NewMemStore(), makeDAG(t), "admin")
	id, _ := s.Create("s", "did:key:alice")
	_ = s.AddMember(id, "did:key:alice", "did:key:bob", "viewer")

	err := s.Kick(id, "did:key:alice", "did:key:bob")
	if err != nil {
		t.Fatalf("Kick: %v", err)
	}

	n, _ := s.store.GetNetwork(id)
	if len(n.Members) != 1 {
		t.Errorf("Members = %d after kick, want 1", len(n.Members))
	}
}

func TestKick_NonAdminCannotKick(t *testing.T) {
	s := NewService(NewMemStore(), makeDAG(t), "admin")
	id, _ := s.Create("s", "did:key:alice")
	_ = s.AddMember(id, "did:key:alice", "did:key:bob", "viewer")

	err := s.Kick(id, "did:key:bob", "did:key:alice")
	if err != ErrNoPermission {
		t.Errorf("expected ErrNoPermission, got %v", err)
	}
}

func TestSetMemberRole(t *testing.T) {
	s := NewService(NewMemStore(), makeDAG(t), "admin")
	id, _ := s.Create("s", "did:key:alice")
	_ = s.AddMember(id, "did:key:alice", "did:key:bob", "viewer")

	err := s.SetMemberRole(id, "did:key:alice", "did:key:bob", "editor")
	if err != nil {
		t.Fatalf("SetMemberRole: %v", err)
	}

	n, _ := s.store.GetNetwork(id)
	if n.MemberRoles["did:key:bob"] != "editor" {
		t.Errorf("bob role = %q, want editor", n.MemberRoles["did:key:bob"])
	}
}

func TestSetMemberRole_NonAdminDenied(t *testing.T) {
	s := NewService(NewMemStore(), makeDAG(t), "admin")
	id, _ := s.Create("s", "did:key:alice")
	_ = s.AddMember(id, "did:key:alice", "did:key:bob", "viewer")

	err := s.SetMemberRole(id, "did:key:bob", "did:key:alice", "viewer")
	if err != ErrNoPermission {
		t.Errorf("expected ErrNoPermission, got %v", err)
	}
}

func TestStore_ListForMember(t *testing.T) {
	s := NewService(NewMemStore(), makeDAG(t), "admin")
	id1, _ := s.Create("s1", "did:key:alice")
	id2, _ := s.Create("s2", "did:key:alice")
	_ = id1
	_ = id2

	ids, err := s.store.ListForMember("did:key:alice")
	if err != nil {
		t.Fatalf("ListForMember: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("ListForMember = %d, want 2", len(ids))
	}
}

func TestStore_CreateGet(t *testing.T) {
	store := NewMemStore()
	n := &Network{
		SuiteID:     "suite-1",
		Members:     []string{"a", "b"},
		MemberRoles: map[string]string{"a": "admin", "b": "viewer"},
	}
	id, err := store.CreateNetwork(n)
	if err != nil {
		t.Fatalf("CreateNetwork: %v", err)
	}

	got, err := store.GetNetwork(id)
	if err != nil {
		t.Fatalf("GetNetwork: %v", err)
	}
	if got == nil {
		t.Fatal("GetNetwork returned nil")
	}
	if got.SuiteID != "suite-1" {
		t.Errorf("SuiteID = %q", got.SuiteID)
	}
	if len(got.Members) != 2 {
		t.Errorf("Members = %d", len(got.Members))
	}
}

func TestStore_GetNotFound(t *testing.T) {
	store := NewMemStore()
	n, err := store.GetNetwork("nonexistent")
	if err != nil {
		t.Fatalf("GetNetwork: %v", err)
	}
	if n != nil {
		t.Error("expected nil for nonexistent network")
	}
}
