# LinkSelf — English (full documentation)

**Documentation:** English (this page) | [日本語（詳細）](README.ja.md)  
**Overview:** [README](../README.md)

---

## Table of contents

- [Central servers are convenient. But…](#central-servers-are-convenient-but)
- [The appeal of going decentralized](#the-appeal-of-going-decentralized)
  - [Scenario: Tanaka and friends](#scenario-tanaka-and-friends)
  - [Supplement: DHT, nodes, malicious actors](#supplement-dht-nodes-malicious-actors)
  - [Summary: What’s great about it](#summary-whats-great-about-it)
- [Technical Approach](#technical-approach)
- [Roadmap](#roadmap)
- [Getting Started](#getting-started)

**Related docs:** [Phase 1 design](phase1-design.en.md), [Group concept](network-concept.en.md), [Sample app plan (chat + file sharing)](sample-chat-app-plan.en.md), [Gitcoin funding procedure](gitcoin-funding.en.md).

---

## Central servers are convenient. But…

With a central server (cloud), you can connect anytime, anywhere: log in, sync data, and communicate with others. The flip side: the central server knows everything about you, holds your data, and controls your connections. There are also **operating costs** (you pay the platform) and **freedom and censorship** (governments can order platforms to disclose information).

---

## The appeal of going decentralized

Imagine a world where your identity and connections are yours alone, with no central server.

### Scenario: Tanaka and friends

1. **DID generation and sharing** — Tanaka starts LinkSelf; a secret key is generated on-device and a DID (e.g. `did:key:z6Mkha...`) is derived. She shares this DID with friends (QR, text, link). The private key never leaves the device.

2. **Adding a contact** — Yuki scans Tanaka’s DID and adds her to “contacts.” This is stored only on Yuki’s device; nothing is sent to a central server.

3. **Starting up and joining the network** — When both run LinkSelf, each registers with the libp2p DHT: “My DID is X, my IP is Y.” Several nodes store this information (with TTL). **Node** = any running LinkSelf instance (phone, PC, etc.); nodes collectively maintain the “phone book”; no central server.

4. **Discovery and connection** — Yuki looks up Tanaka’s DID in the DHT, gets her IP, connects directly, and both sides prove identity with a secret-key challenge–response. Traffic is end-to-end encrypted.

5. **Message sync (store-and-forward)** — If Tanaka is offline, Yuki’s app stores the message locally and delivers it when Tanaka comes online.

6. **Multiple devices** — Same secret key → same DID on phone, PC, tablet. Devices find each other via mDNS (local) or DHT (internet) and sync P2P.

7. **Groups** — Group metadata lives on the DHT; members add/remove by agreement. No single point for a censor to delete the group.

### Supplement: DHT, nodes, malicious actors

- **Does the DHT store “unrelated” data?** Yes. Each node holds only part of the phone book; no one holds “everyone’s” data like a central server.
- **Are non–LinkSelf users on the same network?** The libp2p *infrastructure* can be shared (e.g. with IPFS). Application-layer protocol (e.g. `/linkself/...`) is separate: only LinkSelf nodes take part in LinkSelf traffic. You can also run a LinkSelf-only network (private bootstrap / swarm key).
- **Video calls?** Yes. Once you have libp2p streams, you can run your own media protocol (capture, encode, send, decode, render) on top. WebRTC relies on STUN/TURN/signaling servers, so it’s **not recommended** for LinkSelf’s “no servers” philosophy.
- **Malicious use of others’ hashes?** Every connection requires a signature from the secret key. Without the key, an attacker cannot impersonate; fake DHT entries are possible but **useless**.

DHT works like a distributed “phone book” (Kademlia): data is placed and found by hash; no single point of control. Your device stores: (1) your own info, (2) related nodes (contacts, sync peers), (3) a random slice of the DHT for relay. Privacy: you don’t see others’ traffic or full user lists. Security: reputation, rate limits, TTL, and **signature verification on every connection** (automatic; no user action).

### Summary: What’s great about it

No central server ever appears: DIDs are generated on your device, connections are managed on your device, the DHT is distributed, messages are end-to-end encrypted, groups are on your devices. **Your data, your connections, your community — fully yours.**

---

## Technical Approach

- **DID (did:key)** — Self-sovereign ID derivable offline from a secret key; tamper-proof.
- **libp2p DHT** — Maps DIDs to “where are you now” in a distributed phone book; no central server (same idea as Bitcoin).
- **Discovery** — DHT plus mDNS (same Wi‑Fi), Bluetooth, relay, or manual share (QR, etc.).
- **Identity** — Challenge–response with the secret key on connect; impersonation is rejected automatically.
- **Cross-platform** — Go core usable as a library on each platform; UI chosen per platform (e.g. Electron, Flutter, native).
- **Local-first sync** — Store-and-forward when the peer is online.

---

## Open Source

This project is **open source** (Apache-2.0) and welcomes contributions.

## Support / Donate

We plan to register the project on **Gitcoin Grants** to accept donations. Your support helps sustain development. (URL to be added when the grant page is ready.)

---

## Roadmap

- [x] **Phase 1: Core logic** — Define and implement core logic; run multiple nodes locally and cover various test cases. **Done.** See [Phase 1 design](phase1-design.en.md) ([日本語](phase1-design.md)) and [core/README.md](../core/README.md). Related: [Group concept](network-concept.en.md) ([日本語](network-concept.md)), [Sync DB plan](sync-db-plan.en.md) ([日本語](sync-db-plan.md)), [Group and Sync DB implementation](group-syncdb-implementation.en.md) ([日本語](group-syncdb-implementation.md)), [Sample app plan (chat + file sharing)](sample-chat-app-plan.en.md) ([日本語](sample-chat-app-plan.md)).
- [ ] **Phase 2: Infrastructure** — Infrastructure module; one full-featured sample app (chat + P2P file sharing) that **uses LinkSelf as infrastructure**; group info etc. stored in LinkSelf's DB; **path for using LinkSelf as a library** (public API, packaging, docs).
- [ ] **Phase 3: Platforms** — PC (desktop), Android, iOS.

---

## Getting Started

*This project is in early pre-alpha.*

```bash
git clone https://github.com/SeijiShii/link-self.git
cd link-self
go mod init github.com/SeijiShii/link-self
```

For the full story, FAQ, and diagrams, see [README.ja.md](README.ja.md) (Japanese).
