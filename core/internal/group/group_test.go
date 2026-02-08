package group

import (
	"testing"
)

func TestCreateGroup_RequiresAtLeastTwoMembers(t *testing.T) {
	svc := NewService(NewMemStore())
	_, err := svc.CreateGroup([]string{"did:a"}, nil)
	if err != ErrTooFewMembers {
		t.Errorf("CreateGroup(1 member): got err %v, want ErrTooFewMembers", err)
	}
	_, err = svc.CreateGroup(nil, nil)
	if err != ErrTooFewMembers {
		t.Errorf("CreateGroup(nil): got err %v, want ErrTooFewMembers", err)
	}
	_, err = svc.CreateGroup([]string{"did:a", "did:b"}, nil)
	if err != nil {
		t.Errorf("CreateGroup(2 members): %v", err)
	}
}

func TestCreateGroup_SucceedsWithTwoOrMore(t *testing.T) {
	store := NewMemStore()
	svc := NewService(store)
	id, err := svc.CreateGroup([]string{"did:a", "did:b"}, []string{"did:a"})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if id == "" {
		t.Fatal("CreateGroup returned empty id")
	}
	g, _ := store.GetGroup(id)
	if len(g.Members) != 2 || len(g.Owners) != 1 {
		t.Errorf("group members=%v owners=%v", g.Members, g.Owners)
	}
	// 2-person group can have 0, 1, or 2 owners (spec: 隠匿してよい)
	id2, _ := svc.CreateGroup([]string{"did:x", "did:y"}, nil)
	g2, _ := store.GetGroup(id2)
	if len(g2.Owners) != 0 {
		t.Errorf("2-person group with no owners: got %v", g2.Owners)
	}
}

func TestLeave_RemovesMember(t *testing.T) {
	store := NewMemStore()
	svc := NewService(store)
	id, _ := svc.CreateGroup([]string{"did:a", "did:b", "did:c"}, []string{"did:a"})
	err := svc.Leave(id, "did:b")
	if err != nil {
		t.Fatalf("Leave: %v", err)
	}
	g, _ := store.GetGroup(id)
	if len(g.Members) != 2 {
		t.Errorf("after Leave: members %v", g.Members)
	}
	hasB := false
	for _, d := range g.Members {
		if d == "did:b" {
			hasB = true
			break
		}
	}
	if hasB {
		t.Error("did:b should not be in members after Leave")
	}
}

func TestLeave_DissolvesTwoMemberGroup(t *testing.T) {
	store := NewMemStore()
	svc := NewService(store)
	id, _ := svc.CreateGroup([]string{"did:a", "did:b"}, []string{"did:a"})
	err := svc.Leave(id, "did:b")
	if err != nil {
		t.Fatalf("Leave: %v", err)
	}
	g, _ := store.GetGroup(id)
	if g != nil {
		t.Errorf("2-person group should be dissolved after one leaves, got %+v", g)
	}
}

func TestLeave_NotMember(t *testing.T) {
	svc := NewService(NewMemStore())
	id, _ := svc.CreateGroup([]string{"did:a", "did:b"}, nil)
	err := svc.Leave(id, "did:x")
	if err != ErrNotMember {
		t.Errorf("Leave(non-member): got %v, want ErrNotMember", err)
	}
}

func TestOwner_Kick(t *testing.T) {
	store := NewMemStore()
	svc := NewService(store)
	id, _ := svc.CreateGroup([]string{"did:a", "did:b", "did:c"}, []string{"did:a"})
	err := svc.Kick(id, "did:a", "did:b")
	if err != nil {
		t.Fatalf("Kick: %v", err)
	}
	g, _ := store.GetGroup(id)
	if len(g.Members) != 2 {
		t.Errorf("after Kick: members %v", g.Members)
	}
	for _, d := range g.Members {
		if d == "did:b" {
			t.Error("did:b should be removed after Kick")
		}
	}
}

func TestOwner_KickRequiresOwner(t *testing.T) {
	svc := NewService(NewMemStore())
	id, _ := svc.CreateGroup([]string{"did:a", "did:b", "did:c"}, []string{"did:a"})
	err := svc.Kick(id, "did:b", "did:c")
	if err != ErrNotOwner {
		t.Errorf("Kick by non-owner: got %v, want ErrNotOwner", err)
	}
}

func TestOwner_AppointOwner(t *testing.T) {
	store := NewMemStore()
	svc := NewService(store)
	id, _ := svc.CreateGroup([]string{"did:a", "did:b", "did:c"}, []string{"did:a"})
	err := svc.AppointOwner(id, "did:a", "did:b")
	if err != nil {
		t.Fatalf("AppointOwner: %v", err)
	}
	g, _ := store.GetGroup(id)
	hasB := false
	for _, o := range g.Owners {
		if o == "did:b" {
			hasB = true
			break
		}
	}
	if !hasB {
		t.Errorf("after AppointOwner: owners %v", g.Owners)
	}
}

func TestOwner_SelfDemote(t *testing.T) {
	store := NewMemStore()
	svc := NewService(store)
	id, _ := svc.CreateGroup([]string{"did:a", "did:b", "did:c"}, []string{"did:a", "did:b"})
	err := svc.SelfDemote(id, "did:a")
	if err != nil {
		t.Fatalf("SelfDemote: %v", err)
	}
	g, _ := store.GetGroup(id)
	for _, o := range g.Owners {
		if o == "did:a" {
			t.Error("did:a should not be owner after SelfDemote")
		}
	}
	if len(g.Owners) != 1 && g.Owners[0] != "did:b" {
		t.Errorf("remaining owner should be did:b, got %v", g.Owners)
	}
}

func TestOwner_CannotDemoteOtherOwner(t *testing.T) {
	svc := NewService(NewMemStore())
	id, _ := svc.CreateGroup([]string{"did:a", "did:b", "did:c"}, []string{"did:a", "did:b"})
	err := svc.DemoteOwner(id, "did:a", "did:b")
	if err != ErrCannotDemoteOtherOwner {
		t.Errorf("DemoteOwner(other): got %v, want ErrCannotDemoteOtherOwner", err)
	}
}

func TestLastOwnerLeaving_AutoPromotes(t *testing.T) {
	store := NewMemStore()
	svc := NewService(store)
	id, _ := svc.CreateGroup([]string{"did:a", "did:b", "did:c"}, []string{"did:a"})
	err := svc.SelfDemote(id, "did:a")
	if err != nil {
		t.Fatalf("SelfDemote last owner: %v", err)
	}
	g, _ := store.GetGroup(id)
	if len(g.Owners) == 0 {
		t.Fatal("should have auto-promoted an owner, got 0 owners")
	}
	// Remaining members are did:b, did:c; one of them must be owner
	ownerInMembers := false
	for _, o := range g.Owners {
		for _, m := range g.Members {
			if o == m {
				ownerInMembers = true
				break
			}
		}
	}
	if !ownerInMembers {
		t.Errorf("owner must be in members: owners=%v members=%v", g.Owners, g.Members)
	}
}

func TestLastOwnerLeaving_OneMemberBecomesOwner(t *testing.T) {
	store := NewMemStore()
	svc := NewService(store)
	id, _ := svc.CreateGroup([]string{"did:a", "did:b"}, []string{"did:a"})
	err := svc.SelfDemote(id, "did:a")
	if err != nil {
		t.Fatalf("SelfDemote: %v", err)
	}
	g, _ := store.GetGroup(id)
	if len(g.Owners) != 1 {
		t.Errorf("2-person group: after last owner demotes, expect 1 owner, got %v", g.Owners)
	}
	if g.Owners[0] != "did:b" {
		t.Errorf("only remaining member did:b should be owner, got %v", g.Owners[0])
	}
}

func TestLastOwnerLeaving_ByLeave(t *testing.T) {
	store := NewMemStore()
	svc := NewService(store)
	id, _ := svc.CreateGroup([]string{"did:a", "did:b", "did:c"}, []string{"did:a"})
	err := svc.Leave(id, "did:a")
	if err != nil {
		t.Fatalf("Leave: %v", err)
	}
	g, _ := store.GetGroup(id)
	if len(g.Owners) == 0 {
		t.Fatal("after last owner left, should have auto-promoted, got 0 owners")
	}
}
