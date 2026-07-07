/**
 * Table-level access control using the role DAG. Special permission values
 * "self" and "owner" are not resolved here; they must be handled by the
 * caller with domain-specific logic.
 * Port of core/internal/permission (Go).
 */
import type { RoleDAG } from "./role.js";

/**
 * Read/write/delete access for a table or channel. Each field is a role
 * name, or one of the special values:
 *   - "members": any network member (any role or no role)
 *   - "self":    only the DID that wrote the record (caller must check)
 *   - "owner":   only the record creator (caller must check)
 * Empty string means the operation is not allowed
 * (except via null permissions = allow all).
 */
export interface Permissions {
  read: string;
  write: string;
  delete: string;
}

/**
 * Whether memberRole satisfies the required permission. Returns false for
 * "self" and "owner" (caller must handle these).
 */
export function check(dag: RoleDAG, memberRole: string, required: string): boolean {
  if (required === "self" || required === "owner") return false;
  return dag.hasRole(memberRole, required);
}

/** Whether memberRole can read. Null permissions means allow all. */
export function canRead(dag: RoleDAG, memberRole: string, p: Permissions | null): boolean {
  return p == null ? true : check(dag, memberRole, p.read);
}

/** Whether memberRole can write. Null permissions means allow all. */
export function canWrite(dag: RoleDAG, memberRole: string, p: Permissions | null): boolean {
  return p == null ? true : check(dag, memberRole, p.write);
}

/** Whether memberRole can delete. Null permissions means allow all. */
export function canDelete(dag: RoleDAG, memberRole: string, p: Permissions | null): boolean {
  return p == null ? true : check(dag, memberRole, p.delete);
}
