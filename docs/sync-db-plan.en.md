# Sync DB plan — treating the distributed network as a DB

**English** (this page) | [日本語](sync-db-plan.md)  
**Status:** Core implemented (sync layer, storage interface, in-memory implementation; SQLite reference implementation not yet done)  
**See also:** [Phase 1 design](phase1-design.en.md), [Group concept](group-concept.en.md), [Group and Sync DB implementation](group-syncdb-implementation.en.md)

---

## 1. Goals

- Treat the distributed network as a DB from the app’s perspective.
- **Sync layer wrapping SQLite3** for generality. **Storage is interface-based**; the app provides the implementation. The sync layer is responsible only for “meta attachment, immediate delivery, last-write-wins apply.”
- **App implementers** (1) choose or implement storage (e.g. SQLite reference implementation) and (2) define tables and CRUD against that storage. The sync layer attaches and manages sync meta.
- Use case: a group of users with the app installed edit documents (Figma-style); data is stored in app-chosen storage (e.g. SQLite) and the sync layer shares it over the network.
- **Each record gets meta (ID, groupId, DID, Timestamp)** and is **shared immediately** to devices on the network.
- Edit conflicts are resolved by **last-write-wins**.
- **Edit/view permissions** are handled at the application layer ([Group concept](group-concept.en.md)). The core group handles only member/owner lifecycle.

---

## 2. Current assumptions

- [core/internal/node](core/internal/node/node.go): Send is 1-to-1 (SendMessage) and group (SendToGroup implemented). Groups follow [Group concept](group-concept.en.md); member DID list.
- [core/internal/storeforward](core/internal/storeforward/storeforward.go): Queue when offline; send when peer is detected online.
- Group = **member DID list**. The DID list for a groupId is held in [core/internal/group](core/internal/group) Store; the sync layer obtains it via MemberResolver (wrapping group.Store) ([Group concept](group-concept.en.md)).

---

## 3. Architecture overview

- **App implementers** provide a **storage implementation** (e.g. SQLite reference or custom persistence) and inject it into the sync layer. Table definitions and CRUD are against that storage.
- **Sync layer** treats **storage as an interface**. On write it **attaches meta (ID, groupId, DID, Timestamp)** to each record and **immediately** shares it to members of that groupId via **SendToGroup**. On receive, only the **newer** of two records (same ID or table+pk) is applied to storage (last-write-wins).

---

## 4. Design points

### 4.1 Placement and dependencies

- **New package:** `core/internal/syncdb`. Sync layer (meta, immediate delivery, last-write-wins). Sits between app and `node`.
- **Dependencies:** Use node’s **SendToGroup** and **SetOnMessage**. **Storage is an interface**; the app injects the implementation. Groups follow [Group concept](group-concept.en.md); **groupId → member DID list** is held by the sync layer or app.

### 4.2 Storage interface

- **Storage is interface-based**; the app provides the implementation. The sync layer requires only the minimal operations for “read/write records” and “get/compare Timestamp.”
- **Benefits:** Flexibility (SQLite or custom persistence), testability (in-memory or mock), separation of concerns (sync layer focuses on meta, delivery, last-write-wins).
- **Interface contract:** Put / Get / Delete records; get existing Timestamp for a key (for last-write-wins). Key = record ID or table+primary key.
- **Reference implementation:** Provide a SQLite-based storage implementation; the app may use it as-is or wrap it (e.g. encryption).

### 4.3 Record meta and immediate sharing

| Meta field | Description |
|------------|-------------|
| **ID** | Unique record ID (e.g. UUID). Used for conflict resolution. |
| **groupId** | Which group the record belongs to. Used to decide recipients (member DID list). |
| **DID** | DID of the user (node) that wrote the record. |
| **Timestamp** | Update time (milliseconds). Used for last-write-wins. |

- When a write is committed, **changed records** are serialized with the above meta and **immediately** sent to members of that groupId via **SendToGroup**.
- Payload format: record content + meta (id, groupId, DID, Timestamp) encoded as JSON etc. Receivers apply to storage with **Timestamp-based last-write-wins**.

### 4.4 Group delivery

- Send API: **SendToGroup(ctx, memberDIDs, payload)**. The **member DID list for groupId** is held by the sync layer or app; on commit, **each DID in that group** gets immediate delivery.
- Offline peers receive via existing **Store-and-Forward** when they come online.

### 4.5 Receive, apply, and last-write-wins

- The callback registered with **SetOnMessage** decodes the payload as “record + meta (ID, groupId, DID, Timestamp).”
- **Apply:** Call Put / Delete on the **storage interface** according to the record. The storage implementation (e.g. SQLite) performs the actual INSERT/UPDATE/DELETE.
- **Last-write-wins:** For the same **record ID** (or table+pk), get the existing Timestamp from storage; **apply only if the received record’s Timestamp is greater**. Skip if equal or older.
- On first apply or when meta is missing, apply.

---

## 5. Notes and alignment with other plans

- **Group discovery:** Who belongs to a group follows [Group concept](group-concept.en.md); member list is stored locally on each node. **Member DID list is provided by the app to SyncDB** (invite flow defined by app or a later phase).
- **Permissions:** Document edit/view is handled at the **application layer**. Core group handles only member/owner lifecycle.
- **Order:** Message arrival order and real-time order can differ; **always compare by timestamp** for last-write-wins, not by arrival order.
- **Timestamp:** Implementation may use logical time (e.g. Lamport) for NTP skew; for now use wall-clock milliseconds and keep it extensible.

---

## 6. Summary

- Add a **sync layer** on top of node. **Storage is interface-based**; the app provides the implementation (SQLite reference or custom).
- The sync layer handles **meta attachment, immediate delivery, last-write-wins apply** only. It attaches ID, groupId, DID, Timestamp to each record and **immediately** shares to devices (member DIDs for that groupId) via SendToGroup.
- On receive, compare **per record by Timestamp** and apply via the storage interface with **last-write-wins**.
- Groups follow [Group concept](group-concept.en.md); **groupId → member DID list** is held by the sync layer or app.

---

## 7. Implementation and tests

- **Implementation:** [core/internal/syncdb](../core/internal/syncdb). `RecordStorage` interface and in-memory implementation (`NewMemStorage`), `MemberResolver` (`GroupStoreResolver` wrapping group.Store, excluding self DID), `SyncLayer` (`Put` / `HandleIncoming`). Node has `SendToGroup`. Payload is JSON for `SyncRecord`.
- **Tests:** Unit (`sync_test.go`: meta attachment, store, SendToGroup call, last-write-wins apply, skip older timestamp, Delete on Deleted), integration (`test/integration/syncdb_test.go`: 2 nodes + 1 group, A Puts → B receives and applies to storage).
- **Details:** See “2. Node SendToGroup,” “3. Sync DB,” “4. Integration tests” in [Group and Sync DB implementation](group-syncdb-implementation.en.md).
