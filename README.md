# LinkSelf

**A pure P2P infrastructure for the sovereign individual. No servers, just DID-based existence and direct links.** *主権ある個人のための純粋なP2Pインフラ。サーバーは不要。DIDによる実存の証明とダイレクトな繋がりを。*

---

## 🖋 Philosophy (哲学と美学)

In the modern digital world, our "existence" and "connections" depend on massive central servers. LinkSelf fundamentally overturns this structure.

現代のデジタル世界において、私たちの「実存」や「繋がり」は巨大な中央サーバーに依存しています。LinkSelfはその構造を根本から覆します。

- **Zero Servers:** Data moves directly between devices, bypassing intermediaries. (サーバー・ゼロ: データをデバイス間で直接移動させます)
- **Proof of Existence:** Your private key is your only identity. No phone numbers, no emails. (実存の証明: 秘密鍵のみがアイデンティティを証明します)
- **Autonomous Links:** Devices discover and sync automatically via Bluetooth, Wi-Fi Direct, or across the internet. (自律する繋がり: あらゆるネットワークを介して自律的に同期します)



## 🛠 Technical Approach (技術的アプローチ)

- **Decentralized Identifier (DID):** Uses the `did:key` method for offline-capable self-sovereign identity. (W3C準拠の `did:key` を採用し、オフラインでも自己証明を可能に)
- **libp2p:** Leveraging the world's most robust P2P stack for NAT traversal and encrypted communication. (libp2pによる強力なNAT越えと暗号化通信)
- **Cross-Platform:** Go-based core engine, accessible via Flutter on Android, iOS, and Windows. (Go製コアエンジンとFlutterによるマルチプラットフォーム展開)
- **Local-first Sync:** Store-and-forward logic for peer synchronization when both parties are online. (相手がオンラインになった瞬間に同期する遅延送付ロジック)

## 🚀 Roadmap (ロードマップ)

- [ ] **Phase 1: Core Identity (Go)**
    - Implement `ed25519` key management and `did:key` generation. (`did:key` 生成と鍵管理の実装)
    - Establish basic P2P handshake using libp2p. (libp2pによる基本的なP2P通信の確立)
- [ ] **Phase 2: Mobile Integration (Flutter)**
    - Seamless data sharing from Android "Share Menu". (Android共有メニューからのシームレスなデータ連携)
    - Multi-device auto-sync protocol. (マルチデバイス間の自動同期プロトコル)
- [ ] **Phase 3: Ecosystem**
    - Release SDK for other apps to use LinkSelf for auth/comm. (他アプリ向け認証・通信SDKの公開)
    - Sustainable development via Public Goods funding (e.g., Gitcoin). (Gitcoin等による持続的な開発体制)

## 📦 Getting Started (開発の始め方)

*This project is in early pre-alpha stage.*

```bash
git clone [https://github.com/SeijiShii/link-self.git](https://github.com/SeijiShii/link-self.git)
cd link-self
go mod init [github.com/SeijiShii/link-self](https://github.com/SeijiShii/link-self)