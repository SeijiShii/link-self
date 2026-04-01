package role

import "testing"

func TestNewDAG_Simple(t *testing.T) {
	defs := RoleDefs{
		"viewer": {},
		"nurse":  {Includes: []string{"viewer"}},
		"admin":  {Includes: []string{"nurse"}},
	}
	dag, err := NewDAG(defs)
	if err != nil {
		t.Fatalf("NewDAG: %v", err)
	}
	if dag == nil {
		t.Fatal("NewDAG returned nil")
	}
}

func TestNewDAG_CyclicError(t *testing.T) {
	defs := RoleDefs{
		"a": {Includes: []string{"b"}},
		"b": {Includes: []string{"a"}},
	}
	_, err := NewDAG(defs)
	if err == nil {
		t.Fatal("expected error for cyclic DAG")
	}
}

func TestNewDAG_SelfCycleError(t *testing.T) {
	defs := RoleDefs{
		"a": {Includes: []string{"a"}},
	}
	_, err := NewDAG(defs)
	if err == nil {
		t.Fatal("expected error for self-referencing role")
	}
}

func TestNewDAG_UnknownIncludeError(t *testing.T) {
	defs := RoleDefs{
		"admin": {Includes: []string{"nonexistent"}},
	}
	_, err := NewDAG(defs)
	if err == nil {
		t.Fatal("expected error for unknown included role")
	}
}

func TestHasRole_DirectMatch(t *testing.T) {
	defs := RoleDefs{
		"viewer": {},
		"nurse":  {Includes: []string{"viewer"}},
	}
	dag, _ := NewDAG(defs)

	if !dag.HasRole("nurse", "nurse") {
		t.Error("nurse should satisfy nurse (direct match)")
	}
}

func TestHasRole_Inherited(t *testing.T) {
	defs := RoleDefs{
		"viewer": {},
		"nurse":  {Includes: []string{"viewer"}},
		"admin":  {Includes: []string{"nurse"}},
	}
	dag, _ := NewDAG(defs)

	if !dag.HasRole("admin", "viewer") {
		t.Error("admin should satisfy viewer (transitive)")
	}
	if !dag.HasRole("admin", "nurse") {
		t.Error("admin should satisfy nurse")
	}
	if !dag.HasRole("nurse", "viewer") {
		t.Error("nurse should satisfy viewer")
	}
}

func TestHasRole_NotInherited(t *testing.T) {
	defs := RoleDefs{
		"viewer": {},
		"nurse":  {Includes: []string{"viewer"}},
		"admin":  {Includes: []string{"nurse"}},
	}
	dag, _ := NewDAG(defs)

	if dag.HasRole("viewer", "nurse") {
		t.Error("viewer should NOT satisfy nurse")
	}
	if dag.HasRole("viewer", "admin") {
		t.Error("viewer should NOT satisfy admin")
	}
}

func TestHasRole_DiamondDAG(t *testing.T) {
	// admin includes both nurse and accountant, both include viewer
	defs := RoleDefs{
		"viewer":     {},
		"nurse":      {Includes: []string{"viewer"}},
		"accountant": {Includes: []string{"viewer"}},
		"admin":      {Includes: []string{"nurse", "accountant"}},
	}
	dag, _ := NewDAG(defs)

	if !dag.HasRole("admin", "viewer") {
		t.Error("admin should satisfy viewer (diamond)")
	}
	if !dag.HasRole("admin", "accountant") {
		t.Error("admin should satisfy accountant")
	}
	if dag.HasRole("nurse", "accountant") {
		t.Error("nurse should NOT satisfy accountant (parallel branches)")
	}
	if dag.HasRole("accountant", "nurse") {
		t.Error("accountant should NOT satisfy nurse (parallel branches)")
	}
}

func TestHasRole_SpecialMembers(t *testing.T) {
	defs := RoleDefs{
		"viewer": {},
	}
	dag, _ := NewDAG(defs)

	// "members" is always satisfied by any role
	if !dag.HasRole("viewer", "members") {
		t.Error("any role should satisfy 'members'")
	}
	if !dag.HasRole("", "members") {
		t.Error("empty role should satisfy 'members'")
	}
}

func TestHasRole_UnknownRole(t *testing.T) {
	defs := RoleDefs{
		"viewer": {},
	}
	dag, _ := NewDAG(defs)

	if dag.HasRole("unknown", "viewer") {
		t.Error("unknown member role should not satisfy viewer")
	}
}

func TestNewDAG_Empty(t *testing.T) {
	dag, err := NewDAG(RoleDefs{})
	if err != nil {
		t.Fatalf("NewDAG empty: %v", err)
	}
	// "members" should always work
	if !dag.HasRole("", "members") {
		t.Error("empty DAG should still satisfy 'members'")
	}
}
