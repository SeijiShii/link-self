// Package permission provides table-level access control using the role DAG.
// Special permission values "self" and "owner" are not resolved here;
// they must be handled by the caller with domain-specific logic.
package permission

import "github.com/SeijiShii/link-self/core/internal/role"

// Permissions defines read/write/delete access for a table or channel.
// Each field is a role name, or one of the special values:
//   - "members": any network member (any role or no role)
//   - "self":    only the DID that wrote the record (caller must check)
//   - "owner":   only the record creator (caller must check)
//
// Empty string means the operation is not allowed (except via nil Permissions = allow all).
type Permissions struct {
	Read   string
	Write  string
	Delete string
}

// Check reports whether memberRole satisfies the required permission.
// Delegates to role.DAG.HasRole for role-based checks.
// Returns false for "self" and "owner" (caller must handle these).
func Check(dag *role.DAG, memberRole, required string) bool {
	if required == "self" || required == "owner" {
		return false
	}
	return dag.HasRole(memberRole, required)
}

// CanRead reports whether memberRole can read with the given permissions.
// nil permissions means allow all.
func CanRead(dag *role.DAG, memberRole string, p *Permissions) bool {
	if p == nil {
		return true
	}
	return Check(dag, memberRole, p.Read)
}

// CanWrite reports whether memberRole can write with the given permissions.
// nil permissions means allow all.
func CanWrite(dag *role.DAG, memberRole string, p *Permissions) bool {
	if p == nil {
		return true
	}
	return Check(dag, memberRole, p.Write)
}

// CanDelete reports whether memberRole can delete with the given permissions.
// nil permissions means allow all.
func CanDelete(dag *role.DAG, memberRole string, p *Permissions) bool {
	if p == nil {
		return true
	}
	return Check(dag, memberRole, p.Delete)
}
