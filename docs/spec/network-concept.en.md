# Network concept

**English** (this page) | [日本語](network-concept.md)  
**See also:** [Phase 1 design](phase1-design.en.md), [Sync DB plan](sync-db-plan.en.md), [Data sync concept](data-sync-concept.en.md)

---

## 1. Overview

- **Network** = a set of members (DIDs). **At least 1 member** (can be used as a personal data space).
- **1-to-1** is treated as a "2-person network"; there is no dedicated 1-to-1 concept.
- Network member lists and role information are stored **locally on each node** (no network entity on the DHT).
- **Access control** is managed through a role DAG (directed acyclic graph). The app provides role definitions, and LinkSelf enforces role-based access control.

---

## 2. Where network information is stored

- Network information (member DID list, role assignments) is managed via the interface-based `network.Store`.
- Default implementations: in-memory (`MemStore`) and SQLite3.
- Stored locally on each device; no central server or DHT holds the network entity.

---

## 3. Members and leaving

| Item | Description |
|------|-------------|
| **Members** | List of DIDs in the network. At least 1. |
| **Leave** | A member may leave the network at any time. |
| **1-person network** | A single-member network functions as a personal data space. |
| **2-person network** | When one of two members leaves, the network becomes a 1-person network. |

---

## 4. Role DAG (access control)

The owner concept is **unified into the role DAG**. The role DAG is used for **both network management and table permissions**.

### Scope of the role DAG

> **Important:** The role DAG is used not only for network management operations but also for table-level (MyDB) access control.

| Scope | Description | Implementation |
|-------|-------------|----------------|
| **Network management** | Permission checks for adding/kicking members and changing roles | `network.Service` |
| **Table permissions** | Resolving role names in read/write/delete permissions | `permission.Check` → `DAG.HasRole` |

If a table permission specifies a role name (e.g. `"nurse"`) that is not defined in the DAG, access is **always denied**.

### When `Config.Roles` is nil

Setting `Roles: nil` creates an empty DAG. Behavior of each permission value with an empty DAG:

| Permission value | Behavior | Reason |
|-----------------|----------|--------|
| `nil` (Permissions struct itself is nil) | Allow all | DAG is not consulted |
| `"members"` | Allow all members | Special value; bypasses DAG |
| `"self"` / `"owner"` | Resolved by caller | Does not go through DAG |
| Role names (e.g. `"nurse"`) | **Always denied** | No role definitions in empty DAG |
| `""` (empty string) | Denied | Empty string means operation not allowed |

**Conclusion:** When running with `Roles: nil`, only `"members"` / `"self"` / `"owner"` can be used for table permissions. If fine-grained role-based access control is needed, `Config.Roles` must be configured.

### Role definitions

The app defines the role hierarchy via `Config.Roles`:

```go
Roles: role.RoleDefs{
    "viewer":     {},                              // base role
    "nurse":      {Includes: []string{"viewer"}},  // includes viewer's permissions
    "admin":      {Includes: []string{"nurse"}},   // includes nurse's permissions
}
```

Roles form a directed acyclic graph (DAG); `Includes` defines containment relationships. Cyclic references cause an error.

### Management operation permissions

| Operation | Required role |
|-----------|--------------|
| Add member | `Config.AdminRole` (default: "admin") |
| Kick member | `Config.AdminRole` |
| Change role assignment | `Config.AdminRole` |
| Leave | Anyone (self only) |

### Role assignment to members

- 1 member = 1 role. To combine multiple roles, define a composite role in the DAG.
- Members without an assigned role have minimal permissions (`members` permission only).

---

## 5. Table permissions and sync scope

Table-level permission settings determine the data sync scope:

| read permission | Sync scope | DAG required |
|----------------|------------|-------------|
| `self` | Between the same user's devices only | No |
| `members` | Distributed to all network members | No |
| `<role_name>` | Distributed only to members with the required role or above | **Yes** |

> **Note:** When using `<role_name>`, the role must be defined in `Config.Roles` (see §4).

For details, see [Data sync concept §6](data-sync-concept.en.md).

---

## 6. Permissions are defined by the app layer

- Role definitions are hardcoded by the app. LinkSelf does not know "what a role means."
- Table-level read/write/delete permissions are set via `MyDB.SetPermissions()`.
- Row-level access control is not provided by LinkSelf. The app controls it with WHERE clauses.

---

## 7. Relation to other documents

- **Phase 1 design** ([phase1-design.en.md](phase1-design.en.md)): Network is in Phase 1 scope.
- **Sync DB plan** ([sync-db-plan.en.md](sync-db-plan.en.md)): DeviceSync / GroupShare two-layer architecture.
- **Data sync concept** ([data-sync-concept.en.md](data-sync-concept.en.md)): Suite / Network two-layer concept, permission model, SQL interface.
- **Sample chat app** ([sample-chat-app-plan.en.md](../app/sample-chat-app-plan.en.md)): Sample app design.

---

## 8. Implementation

- **Implementation**: `core/internal/network`. Store is defined by the `Store` interface; in-memory (`MemStore`) and SQLite implementations are included.
- **Domain logic**: `Service` (`Create`, `AddMember`, `Leave`, `Kick`, `SetMemberRole`). All management operations perform `AdminRole` role DAG permission checks.
- **Legacy**: `core/internal/group` is the old owner-based implementation. `core/internal/network` is the successor.
- **Tests**: Store unit tests, service unit tests (including role permission checks).
