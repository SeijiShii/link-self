# Group concept

**English** (this page) | [日本語](group-concept.md)  
**See also:** [Phase 1 design](phase1-design.en.md), [Sync DB plan](sync-db-plan.en.md)

---

## 1. Overview

- **Group** = a set of members (DIDs). **At least 2 members**.
- **1-to-1** is treated as a “2-person group”; there is no dedicated 1-to-1 concept.
- Group member list and owner information are stored **locally on each node** (no group entity on the DHT).
- **Edit/view permissions** and similar are handled at the **application layer**, not by the network group concept. The core group handles only “who is a member” and “who is an owner”; the app defines permission details.

---

## 2. Where group information (groups I belong to) is stored

- **Provided as infrastructure.** Storage of group information (list of groups I belong to, member DIDs per group, owner info) is **interface-based**; the implementation lives in the infrastructure. **Eventually implemented with SQLite3 etc., but callers depend only on the interface.**
- **Interface:** Define operations for listing/adding/updating/deleting groups and for getting/storing “groupId → member DID list, owner info.” Core, sync layer, and app access group information only through this interface.
- **Implementation:** The infrastructure includes a **default implementation using SQLite3**. Schema (group table, members, owners) is defined in the infrastructure or documented as a convention. The app may use this implementation as-is or inject an implementation that satisfies the interface (in-memory for tests, another DB, etc.).
- **Storage location:** Local to each device (node). No central server or DHT holds the group entity (Phase 1 assumption). For the SQLite3 implementation, use an app-specified DB path (or a default path).
- **Benefits:** (1) Interface allows tests to use mocks or in-memory implementations. (2) Changing the persistence backend (different DB, encryption layer, etc.) is a matter of swapping the implementation. (3) Core and sync layer do not depend on group persistence details.

---

## 3. Members and leaving

| Item | Description |
|------|-------------|
| **Members** | List of DIDs in the group. At least 2. |
| **Leave** | A member may **leave** the group at any time. |
| **2-person group** | When there are 2 members, if one leaves the group is **dissolved**. |

---

## 4. Owner (administrator)

| Item | Description |
|------|-------------|
| **Owner** | A group has the concept of **owner** (administrator). |
| **2-person case** | For 2 members, the owner concept may be **hidden** (if one leaves the group dissolves, so no need to distinguish in practice). |
| **When inviting a 3rd** | When inviting a 3rd member, decide **which of the first two (or both)** become owners. |

### Owner permissions

| Permission | Description |
|------------|-------------|
| **Remove member** | Owner may remove a member (kick). |
| **Appoint owner** | Owner may appoint another member as owner. |
| **Self-demote** | Owner may demote themselves to member. |
| **Demote other owner** | Owner **cannot demote another owner**. |

---

## 5. When the last owner demotes or leaves

- When the **last** owner **self-demotes** or **leaves**, there would be 0 owners.
- In that case **automatically promote at least one of the remaining members to owner** (never leave the group with 0 owners).

### How to choose the new owner(s)

| Method | Description |
|--------|-------------|
| **Arbitrary** | Choose from remaining members by any rule (e.g. nomination, join order). Prefer not to promote only “the last one” (e.g. if 2+ remain, promote 2). |
| **Random** | Pick one of the remaining members at random as owner. |

---

## 6. Permissions are application-layer

- **Document edit/view** and resource-level access control are done at the **application layer**, not by the network group concept.
- The core group handles only the **group lifecycle**: members, owners, leave, dissolve, invite, kick.
- The app layers its own permission model (editable / view-only, per-document, etc.) on top of the group (members, owners).

---

## 7. Relation to other documents

- **Phase 1 design** ([phase1-design.en.md](phase1-design.en.md)): Group is in Phase 1 scope. Send/connect APIs are group-based (SendToGroup / ConnectToGroup).
- **Sync DB plan** ([sync-db-plan.en.md](sync-db-plan.en.md)): Synced groups are “groupId → member DID list”; aligned with the group concept.
- **Sample chat app plan** ([sample-chat-app-plan.en.md](../app/sample-chat-app-plan.en.md)): Uses SendToGroup / ConnectToGroup for the 2-person group case.

---

## 8. Implementation and tests

- **Implementation:** [core/internal/group](../core/internal/group). Store is defined by the `Store` interface; in-memory implementation `NewMemStore` is included. Domain logic is in `Service` (`CreateGroup`, `Leave`, `Kick`, `AppointOwner`, `SelfDemote`, `DemoteOwner`). SQLite3 implementation is not yet provided (can be added later as infrastructure).
- **Tests:** Store unit tests (`store_test.go`: Create/Get/List/Update/Delete/GetNotFound/unique ID), domain unit tests (`group_test.go`: member count, leave, dissolve, owner permissions, no demote-other-owner, auto-promote when last owner leaves).
- **Details:** See “1. Group” in [Group and Sync DB implementation](group-syncdb-implementation.en.md).
