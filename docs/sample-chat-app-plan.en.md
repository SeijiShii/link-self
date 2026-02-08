# Sample app plan (chat + file sharing)

**English** (this page) | [日本語](sample-chat-app-plan.md)  
**Status:** Not yet implemented (plan only, stored in docs)  
**Summary:** Design for one full-featured sample app that **uses LinkSelf as infrastructure**: group/1-to-1 chat and P2P file sharing. Group info, contacts, sync meta, etc. are stored in **LinkSelf's DB**; the app uses LinkSelf's API.  
**See also:** [Group concept](group-concept.en.md), [Phase 1 design](phase1-design.en.md), [Roadmap](README.en.md#roadmap)

---

## Role and data

- **One app, LinkSelf as infrastructure:** The sample app is **one application** that embeds LinkSelf as a library and uses it as the P2P infrastructure. It is the first concrete example of "using LinkSelf from another app."
- **Data in LinkSelf's DB:** **Group info, contacts, sync metadata**, and similar data are stored in **LinkSelf's store (DB)**. The app reads and updates them via LinkSelf's API (e.g. group.Store, SyncLayer). The app does not hold a separate copy of group membership; it relies on LinkSelf.

---

## Conclusion

- **Full-featured sample app (chat + file sharing):** Feasible. One app uses core Node API (`New`, `Start`, `SetOnMessage`, `SendToGroup`, `ConnectToGroup`) and SyncLayer/RecordStorage as provided by LinkSelf. Treat 1-to-1 as a **2-person group** ([Group concept](group-concept.en.md)).
- **Multiple nodes on one PC:** Possible. **One process = one node**; run the same binary multiple times (multiple terminals) on the same PC to simulate multiple nodes. Integration tests [core/test/integration/integration_test.go](../core/test/integration/integration_test.go) already run multiple nodes (different ports) on the same host and verify DHT, auth, and messages.

---

## Architecture

- Each process runs `node.New` → `node.Start` to start one node.
- Pass the first node’s “Listen address + PeerID” as `BootstrapPeers` for the second and later (same as integration tests).
- On the same machine, use `ListenAddrs: ["/ip4/127.0.0.1/tcp/0"]` for automatic port assignment so multiple nodes do not conflict.

---

## Implementation approach

### Placement

- **Phase 1 design** marks `core/cmd/linkself/` as the sample app entry (chat + file sharing, not yet implemented). Put the sample app entry point in **`core/cmd/linkself/`** (one binary that embeds LinkSelf and provides chat + file sharing).

### Minimal features

| Item | Description |
|------|-------------|
| Start | One process starts one node. `ListenAddrs: /ip4/127.0.0.1/tcp/0` (optional port). |
| Identity | First run: generate with `did.Generate()` and save locally (e.g. `identity.json` or libp2p key format). Later: load. |
| Bootstrap | Flag for “first node’s address” (e.g. `--bootstrap /ip4/127.0.0.1/tcp/xxxx/p2p/<PeerID>`). Second and later join DHT via this. |
| Contacts | Phase 1 “store only”: keep DIDs in a local file or in-memory list. |
| Receive | Print payload received via `SetOnMessage` to stdout (plain string or simple JSON). |
| Send | Interactively enter “group (member DID list)” and “body” → `ConnectToGroup(ctx, memberDIDs)` for auth and flush, then `SendToGroup(ctx, memberDIDs, []byte(text))`. For 2-person chat use 2-person group `[myDID, peerDID]`. |
| Exit | Call `Close()` then exit process. |

### File sharing (scope)

- **In scope:** P2P file send/receive or group-based file sharing (e.g. shared folder sync). File metadata (name, size, hash, chunk IDs) can be shared via SyncLayer; chunk payload via a separate protocol (e.g. Node stream). Details (chunk size, retry, partial fetch) to be decided at implementation time.
- **Features to add:** File send, receive, list, delete; offline queue for file chunks.

### Message format

- Core sends/receives **arbitrary `[]byte`**. In the sample, use “UTF-8 text line” or minimal JSON like `{"from":"<DID>","text":"..."}`.

### Usage (one PC, two nodes, 2-person group)

1. **Terminal 1:** `go run ./cmd/linkself` (or built `linkself`) → prints “My DID: did:key:...”, “Listen: /ip4/127.0.0.1/tcp/4001/p2p/...”.
2. **Terminal 2:** `linkself --bootstrap /ip4/127.0.0.1/tcp/4001/p2p/<PeerID>` → second node joins DHT. Add terminal 1’s DID to contacts, register **2-person group** (self + peer DIDs), then `connectToGroup` and `sendToGroup`.
3. On both sides: “add peer DID → connectToGroup as 2-person group → sendToGroup” for bidirectional chat. Store-and-Forward is handled by core; offline send works as-is.

---

## Implementation tasks (draft)

1. **Add `core/cmd/linkself/main.go`**  
   Flags: `--listen`, `--bootstrap`, `--identity`. Generate/save/load identity. Pass `ListenAddrs` and `BootstrapPeers` to `node.New` → `Start`. Print “My DID”, “Listen address” on start.

2. **Interactive loop**  
   Read commands from stdin (e.g. `add <DID>`, `group <DID>...`, `connectToGroup`, `sendToGroup <text>`, `list`, `quit`). `SetOnMessage`: print “From <DID>: <text>”. `add`: add DID to contacts. `group`: register current group (member DID list). `connectToGroup`: call `node.ConnectToGroup(ctx, memberDIDs)`. `sendToGroup`: call `node.SendToGroup(ctx, memberDIDs, []byte(text))`. `quit`: `node.Close()` and exit.

3. **README / docs**  
   Add “How to run the sample chat” to `core/README.md` or `docs/`. Describe: start two terminals on one PC, exchange bootstrap and DID, then chat.

---

## Notes and dependencies

- **internal:** `cmd/linkself` is inside the `core` module, so it can import `internal/did`, `internal/node`, `internal/dht`, etc.
- **DHT stability:** As in integration tests, the second node may take a few seconds to find the first in the DHT. Consider “retry after a few seconds on connect failure.”
- **Key persistence:** Use libp2p `crypto.MarshalPrivateKey` / `crypto.UnmarshalPrivateKey` and `did.FromPrivKey` to align with the existing did package.

---

## Summary

- **One full-featured sample app** (chat + file sharing) uses **LinkSelf as infrastructure**. Group info, contacts, sync meta are stored in **LinkSelf's DB**; the app uses LinkSelf's API.
- Send/receive is **group-based** (SendToGroup / ConnectToGroup); 1-to-1 is a 2-person group.
- **Multiple nodes on one PC** is done by running the same binary in multiple processes (multiple terminals) and connecting via bootstrap.
- Implementation scope: one entry point + chat + file sharing + identity persistence + group management (via LinkSelf) + short documentation. GUI (Web / desktop / TUI) to be decided at implementation time.

## Related documents

- [Group concept](group-concept.en.md): Group, owner, leave, permissions.
- [Phase 1 design](phase1-design.en.md): Core scope and API approach.
- [Sync DB plan](sync-db-plan.en.md): Treating the network as a DB.
