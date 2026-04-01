# Group and Sync DB implementation

**English** (this page) | [日本語](group-syncdb-implementation.md)  
**See also:** [Group concept](network-concept.en.md), [Sync DB plan](sync-db-plan.en.md)

This document summarizes **implementation layout, APIs, and tests** in line with the above plans.

> **Note (2026-03):** Sync DB has been migrated to the **DeviceSync / GroupShare two-layer architecture**. The old `syncdb` package (§3) is being replaced by `devicesync` + `groupshare`. See [sync-db-plan.en.md](sync-db-plan.en.md) for the new architecture.

---

## 1. Group (core/internal/group)

### 1.1 Plan vs implementation

| Plan (group-concept) | Implementation |
|----------------------|----------------|
| Group = member DID set, at least 2 | `Group.Members`, `Service.CreateGroup` returns `ErrTooFewMembers` for &lt; 2 |
| 1-to-1 = 2-person group | No dedicated type; create with 2 members |
| Store is interface-based | `Store` interface; callers depend only on it |
| Member leave; 2-person group dissolves | `Service.Leave`; when 1 member remains, `DeleteGroup` |
| Owner: kick, appoint, self-demote; cannot demote other owner | `Service.Kick`, `AppointOwner`, `SelfDemote`, `DemoteOwner` (other owner → `ErrCannotDemoteOtherOwner`) |
| When last owner leaves/demotes, auto-promote | `autoPromoteOneExcluding` promotes one remaining member to owner |

### 1.2 Package layout

| File | Content |
|------|---------|
| `store.go` | `Group` (ID, Members, Owners), `Store` interface, `ErrGroupNotFound` |
| `memstore.go` | In-memory `Store` (UUID groupID). For tests/validation |
| `group.go` | `Service` and domain logic. `ErrTooFewMembers`, `ErrNotMember`, `ErrNotOwner`, `ErrCannotDemoteOtherOwner`, `ErrTargetNotMember` |

### 1.3 Main API

- **Store:** `ListGroupIDsForMember`, `GetGroup`, `CreateGroup`, `UpdateGroup`, `DeleteGroup`
- **Service:** `NewService(store)`, `CreateGroup`, `Leave`, `Kick`, `AppointOwner`, `SelfDemote`, `DemoteOwner`

### 1.4 Tests (core/internal/group)

**Store (store_test.go)**

| Test | Description |
|------|-------------|
| `TestMemStore_CreateGet` | After Create, Get returns same content |
| `TestMemStore_ListByMember` | List groups by member DID |
| `TestMemStore_Update` | After Update, Get reflects changes |
| `TestMemStore_Delete` | After Delete, Get is nil and List no longer includes it |
| `TestMemStore_GetNotFound` | Get for missing ID returns nil |
| `TestMemStore_CreateReturnsUniqueIDs` | Multiple Create yields unique IDs |

**Domain (group_test.go)**

| Test | Description |
|------|-------------|
| `TestCreateGroup_RequiresAtLeastTwoMembers` | Create with 1 or nil fails |
| `TestCreateGroup_SucceedsWithTwoOrMore` | Create with 2+ succeeds; 2-person with 0 owners allowed |
| `TestLeave_RemovesMember` | In 3-person group, one leave removes that member |
| `TestLeave_DissolvesTwoMemberGroup` | In 2-person group, one leave deletes group |
| `TestLeave_NotMember` | Leave by non-member returns `ErrNotMember` |
| `TestOwner_Kick` | Owner can kick member |
| `TestOwner_KickRequiresOwner` | Non-owner kick returns `ErrNotOwner` |
| `TestOwner_AppointOwner` | Owner can appoint another member as owner |
| `TestOwner_SelfDemote` | Owner can self-demote |
| `TestOwner_CannotDemoteOtherOwner` | Demote other owner returns `ErrCannotDemoteOtherOwner` |
| `TestLastOwnerLeaving_AutoPromotes` | When last owner self-demotes, one remaining is promoted |
| `TestLastOwnerLeaving_OneMemberBecomesOwner` | In 2-person group, when last owner demotes, the other becomes owner |
| `TestLastOwnerLeaving_ByLeave` | When last owner leaves, auto-promote still applies |

Run: `go test ./internal/group/...`

---

## 2. Node SendToGroup (core/internal/node)

### 2.1 Plan alignment

- [phase1-design.en.md](phase1-design.en.md) / [sync-db-plan.en.md](sync-db-plan.en.md): Send API is group-based (SendToGroup).

### 2.2 Implementation

- **SendToGroup(ctx, memberDIDs, payload)**  
  Excludes this node’s `Identity.DID` from memberDIDs and calls existing `SendMessage(ctx, did, payload)` for each DID. Offline peers are queued by Store-and-Forward.

---

## 3. Sync DB (core/internal/syncdb)

### 3.1 Plan vs implementation

| Plan (sync-db-plan) | Implementation |
|---------------------|----------------|
| Meta, immediate delivery, last-write-wins | `SyncLayer.Put` attaches meta, stores, SendToGroup; `HandleIncoming` applies last-write-wins |
| Storage is interface-based | `RecordStorage` interface; app injects implementation |
| Meta: ID, groupId, DID, Timestamp | `SyncRecord`: ID, GroupID, DID, Timestamp, Body, Deleted |
| groupId → member DIDs per group concept | `MemberResolver`; implementation `GroupStoreResolver` (group.Store + exclude self DID) |
| Payload encoded as JSON etc. | `SyncRecord` serialized as JSON for send/receive |
| Last-write-wins by Timestamp | Get existing via `GetTimestamp`; Put only if received Timestamp &gt; existing |
| Delete handling | `SyncRecord.Deleted`; when true, receiver applies Delete |

### 3.2 Package layout

| File | Content |
|------|---------|
| `record.go` | `SyncRecord` (ID, GroupID, DID, Timestamp, Body, Deleted) with JSON tags |
| `storage.go` | `RecordStorage` interface (Put, Get, GetTimestamp, Delete), `ErrNotFound` |
| `memstorage.go` | In-memory `RecordStorage`. For tests/validation |
| `resolver.go` | `MemberResolver` (MemberDIDsForGroup). `GroupStoreResolver` (wraps group.Store, excludes self DID) |
| `sync.go` | `SyncLayer`. `NewSyncLayer(storage, resolver, sendGroup, selfDID)`. `Put`, `PutRecord`, `HandleIncoming` |

### 3.3 Main API

- **RecordStorage:** Put, Get, GetTimestamp, Delete
- **MemberResolver:** `MemberDIDsForGroup(ctx, groupID) ([]string, error)`
- **SyncLayer:** `Put(ctx, groupID, body)` → attach meta, store, immediate delivery. `HandleIncoming(ctx, payload)` → decode, last-write-wins Put/Delete

### 3.4 Tests (core/internal/syncdb)

| Test | Description |
|------|-------------|
| `TestSyncLayer_Put_AttachesMetaAndStores` | Put attaches ID/DID/Timestamp, stores, calls SendToGroup with meta in payload |
| `TestSyncLayer_HandleIncoming_LastWriteWins_NewerApplied` | Newer Timestamp record is applied |
| `TestSyncLayer_HandleIncoming_LastWriteWins_OlderSkipped` | Older Timestamp record is not overwritten |
| `TestSyncLayer_HandleIncoming_DeletedAppliesAsDelete` | Deleted record causes Delete on receiver |

Run: `go test ./internal/syncdb/...`

---

## 4. Integration tests (core/test/integration)

### 4.1 Group and Sync DB (syncdb_test.go)

| Test | Description |
|------|-------------|
| `TestSyncDB_PutOnA_ReceivedOnB` | 2 nodes (A, B), 1 group (A/B as members in group.Store). Each node has MemStorage + SyncLayer. B’s SetOnMessage calls SyncLayer.HandleIncoming. A does Put(groupID, "hello from A"); B’s storage is verified to have the same record. |

Run: `go test ./test/integration/... -run TestSyncDB`

### 4.2 Relation to existing integration tests

- `TestDIDGenerationConsistency`, `TestDHTProvideFind`, `TestConnectAuth`, `TestMessageOnline`, `TestStoreAndForward` unchanged. All pass after adding SendToGroup.

---

## 5. Plan consistency

- **group-concept:** §1–§7 (at least 2 members, leave/dissolve, owner permissions, no demote-other-owner, auto-promote when last owner, permissions at app layer) are covered by implementation and tests.
- **sync-db-plan:** Meta, immediate delivery, last-write-wins, storage interface, groupId → member DIDs (via group.Store), use of SendToGroup/SetOnMessage are implemented. SQLite reference and schema conventions are not (per plan, app or separate task).

This document aligns the plans with the implementation and tests.
