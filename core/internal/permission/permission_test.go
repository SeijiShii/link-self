package permission

import (
	"testing"

	"github.com/SeijiShii/link-self/core/internal/role"
)

func makeDAG(t *testing.T) *role.DAG {
	t.Helper()
	dag, err := role.NewDAG(role.RoleDefs{
		"viewer":     {},
		"nurse":      {Includes: []string{"viewer"}},
		"accountant": {Includes: []string{"viewer"}},
		"admin":      {Includes: []string{"nurse", "accountant"}},
	})
	if err != nil {
		t.Fatalf("NewDAG: %v", err)
	}
	return dag
}

func TestCheck_Members(t *testing.T) {
	dag := makeDAG(t)
	// "members" should be satisfied by any role
	if !Check(dag, "viewer", "members") {
		t.Error("viewer should satisfy 'members'")
	}
	if !Check(dag, "", "members") {
		t.Error("empty role should satisfy 'members'")
	}
}

func TestCheck_RoleMatch(t *testing.T) {
	dag := makeDAG(t)

	if !Check(dag, "nurse", "nurse") {
		t.Error("nurse should satisfy nurse (direct)")
	}
	if !Check(dag, "admin", "nurse") {
		t.Error("admin should satisfy nurse (transitive)")
	}
	if !Check(dag, "admin", "viewer") {
		t.Error("admin should satisfy viewer (transitive)")
	}
}

func TestCheck_RoleNotMatch(t *testing.T) {
	dag := makeDAG(t)

	if Check(dag, "viewer", "nurse") {
		t.Error("viewer should NOT satisfy nurse")
	}
	if Check(dag, "nurse", "accountant") {
		t.Error("nurse should NOT satisfy accountant (parallel)")
	}
}

func TestCheck_Self(t *testing.T) {
	dag := makeDAG(t)
	// "self" is not checked by Check — always returns false
	// (caller handles self-check separately)
	if Check(dag, "admin", "self") {
		t.Error("Check should return false for 'self' (handled by caller)")
	}
}

func TestCheck_Owner(t *testing.T) {
	dag := makeDAG(t)
	// "owner" is not checked by Check — always returns false
	if Check(dag, "admin", "owner") {
		t.Error("Check should return false for 'owner' (handled by caller)")
	}
}

func TestPermissions_CanRead(t *testing.T) {
	dag := makeDAG(t)
	p := &Permissions{Read: "viewer", Write: "nurse", Delete: "owner"}

	if !CanRead(dag, "viewer", p) {
		t.Error("viewer should be able to read (Read=viewer)")
	}
	if !CanRead(dag, "admin", p) {
		t.Error("admin should be able to read (includes viewer)")
	}
}

func TestPermissions_CanWrite(t *testing.T) {
	dag := makeDAG(t)
	p := &Permissions{Read: "viewer", Write: "nurse", Delete: "owner"}

	if !CanWrite(dag, "nurse", p) {
		t.Error("nurse should be able to write")
	}
	if !CanWrite(dag, "admin", p) {
		t.Error("admin should be able to write (includes nurse)")
	}
	if CanWrite(dag, "viewer", p) {
		t.Error("viewer should NOT be able to write")
	}
	if CanWrite(dag, "accountant", p) {
		t.Error("accountant should NOT be able to write (parallel to nurse)")
	}
}

func TestPermissions_Nil(t *testing.T) {
	dag := makeDAG(t)
	// nil permissions = allow all
	if !CanRead(dag, "viewer", nil) {
		t.Error("nil permissions should allow read")
	}
	if !CanWrite(dag, "viewer", nil) {
		t.Error("nil permissions should allow write")
	}
	if !CanDelete(dag, "viewer", nil) {
		t.Error("nil permissions should allow delete")
	}
}

func TestPermissions_MembersDefault(t *testing.T) {
	dag := makeDAG(t)
	p := &Permissions{Read: "members", Write: "members", Delete: "members"}

	if !CanRead(dag, "", p) {
		t.Error("any member should satisfy 'members' read")
	}
	if !CanWrite(dag, "", p) {
		t.Error("any member should satisfy 'members' write")
	}
}
