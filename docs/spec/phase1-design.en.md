# Phase 1: Core logic — design and implementation

**English** (this page) | [日本語](phase1-design.md)  
**Status:** Implemented (including local multi-node tests)  
**See also:** [Roadmap](README.en.md#roadmap), [core/README.md](../core/README.md)

---

## 1. Phase 1 scope

Based on the “Tanaka and friends” scenario and Technical Approach, Phase 1 implements the following core.

| Feature | Description |
|---------|-------------|
| **DID** | Generate and share `did:key` from a secret key on device. Secret key never leaves the device. |
| **Contacts** | Register peer DIDs locally (Phase 1: “store” only). |
| **Startup, DHT registration** | On node start, provide “DID ↔ my address” to libp2p DHT. |
| **Discovery** | Look up peer DID in DHT and get address (PeerID / Multiaddr). |
| **Connect, identity** | Connect to that address and authenticate with secret-key challenge–response. Reject impersonation. |
| **Messages** | Store-and-Forward. Send by group (to each member DID except self). If peer is offline, hold locally and send when online. |
| **Group** | Set of member DIDs (2+). 1-to-1 is a 2-person group. Member list is stored locally on each node. See [Group concept](network-concept.en.md). |

Note: Send/connect APIs are planned to be group-based (SendToGroup / ConnectToGroup). Current implementation is 1-to-1 (SendMessage / Connect).

---

## 2. Architecture (core)

- **DID:** Secret key ↔ `did:key` generation and verification. Ed25519; interoperable with libp2p PeerID.
- **Host + DHT:** One node = one libp2p Host. DHT provides/finds “DID key → PeerInfo.”
- **Auth:** After stream is established, send challenge; peer signs with secret key; verify. Only then treat as authenticated peer.
- **Store-and-Forward:** If recipient is offline, queue locally. Send queue when peer connects (auth) or when Connect succeeds.

---

## 3. Repository layout (implemented)

Core modules live under **`core/`**. Root holds docs, README, LICENSE; Go code is under `core/`.

```
link-self/
├── README.md
├── LICENSE
├── docs/
│   ├── README.ja.md
│   ├── README.en.md
│   └── phase1-design.md    # this document
└── core/                    # core module (Go). Shared across PC / Android / iOS
    ├── go.mod               # module github.com/SeijiShii/link-self/core
    ├── README.md
    ├── internal/
    │   ├── did/             # did:key generation, parse, verify
    │   ├── dht/             # DHT Provide/Find wrap (DID ↔ AddrInfo)
    │   ├── auth/            # challenge–response auth
    │   ├── node/            # node = Host + DHT + Auth + Store
    │   ├── group/           # group concept (Store, Service)
    │   ├── syncdb/          # sync layer (RecordStorage, SyncLayer)
    │   └── storeforward/   # message queue and send on peer online
    ├── pkg/                 # optional public API (unused in Phase 1)
    ├── cmd/linkself/        # Sample app entry (chat + file sharing, not yet implemented)
    └── test/integration/    # local multi-node tests
```

- **go.mod** is only under **core/**. Module path: `github.com/SeijiShii/link-self/core`.
- Phase 2+ (apps on each platform—Flutter, Electron, native, etc.—or other repos) will import this `core` module (gomobile / FFI / subprocess will also target `core/`).

---

## 4. Package roles and dependencies

1. **did** — Key generation, `did:key` generation/parse (Ed25519). Aligned with libp2p key type; PeerID ↔ DID.
2. **dht** — libp2p Host and Kademlia DHT. Provide/Find PeerInfo by DID key. Key design: SHA256(DID string) base32-encoded.
3. **auth** — After stream is established, initiator sends random challenge; responder signs with secret key; initiator verifies with DID/public key. Close stream on failure.
4. **node** — Assembles Host + DHT + Auth. Connect is group-based (ConnectToGroup) in design; currently `Connect(did)` does “find DID → connect → auth.” Store-and-Forward flushes on Connect success or incoming auth completion.
5. **storeforward** — Holds messages per destination DID in memory. On authenticated peer connect or Connect success, sends queued messages for that DID. Re-queues on send failure.

---

## 5. DID ↔ DHT key, message format (spec)

- **DID:** `did:key:` + multibase(base58btc, multicodec(0xed) + raw Ed25519 public key 32 bytes).
- **DHT:** Always uses the public DHT (`ProtocolPrefix("/ipfs")`). Peers are discovered via FindPeer(DIDToPeerID(did)); PutDID/FindDID are not used.
- **Message protocol:** `/linkself/msg/1.0.0`. Payload: 4-byte BigEndian length + body.

---

## 6. Dependencies (used in implementation)

- **libp2p:** `github.com/libp2p/go-libp2p`, `github.com/libp2p/go-libp2p-kad-dht`
- **DID / keys:** libp2p `core/crypto` (Ed25519), `github.com/multiformats/go-multibase`
- **DHT record:** `github.com/libp2p/go-libp2p-record`
- **Tests:** Standard `testing` (integration tests in `test/integration`)

---

## 7. Phase 1 deliverables and completion (done)

- **Core logic definition:** Package layout and DID↔DHT key, message format documented in [core/README.md](../core/README.md) and this document.
- **Implementation:** `core/internal/did`, `dht`, `auth`, `node`, `storeforward`, `group`, `syncdb` satisfy the scope.
- **Local multi-node tests:** 5 scenarios covered by `go test ./test/integration/...`.

### 5 scenarios (covered by tests)

1. **DID generation consistency** — Same secret key yields same DID.
2. **DHT Provide/Find** — Node A provides; Node B finds A’s address by DID.
3. **Connect and auth** — B connects to A by DID; challenge–response auth succeeds.
4. **Message (online)** — A and B online; message B→A is received by A.
5. **Message (Store-and-Forward)** — B sends to A while A is offline; B queues; A starts; B Connect flushes; A receives.

---

## 8. Relation to Phase 2+

Phase 2 (infrastructure module, sample apps) will extend the Phase 1 `core` module (APIs under `core/internal`). Root [README.md](../README.md) Getting Started may be updated in Phase 2 to run `cd link-self/core` and `go mod tidy` etc.

## 9. Related documents

- [Group concept](network-concept.en.md): Group, owner, leave, permissions.
- [Sync DB plan](sync-db-plan.en.md): Treating the network as a DB from the app.
- [Sample chat app plan](../app/sample-chat-app-plan.en.md): Sample app design using group APIs.
