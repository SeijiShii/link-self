# LinkSelf リポジトリ — 中央 TODO

コンテキストの分断を避けるため、**リポジトリ全体**の現状と今後の作業をここにまとめる。

---

## 1. プロジェクト概要

- **LinkSelf**: 主権ある個人のための**サーバー不要の P2P インフラ**。DID（did:key）、libp2p DHT、Store-and-Forward、エンドツーエンド暗号化。
- **リポジトリ**: Go コア（ライブラリ＋daemon）、サンプルチャットアプリ（Electron）、設計・計画ドキュメントを包含。
- **ライセンス**: Apache-2.0。Gitcoin Grants での寄付募集を予定。

---

## 2. リポジトリ構成

```
link-self/
├── TODO.md                    # 本ファイル（リポジトリ全体の中央 TODO）
├── README.md                   # 概要・ロードマップ・Getting Started
├── LICENSE
├── core/                       # Go コア（ライブラリ＋daemon）
│   ├── go.mod
│   ├── internal/               # 実装詳細（外部から import 不可）
│   │   ├── did/                # did:key 生成・検証（Ed25519）
│   │   ├── dht/                # DHT Provide/Find ラップ
│   │   ├── auth/               # チャレンジ・レスポンス認証
│   │   ├── node/               # ノード（Host + DHT + Auth + Store-and-Forward）
│   │   ├── storeforward/       # メッセージキュー・オンライン検知時送信
│   │   ├── group/              # グループ（メンバー DID 集合）・ストア
│   │   ├── devicesync/         # DeviceSync（同一DID間の透過的DB同期）✅ 実装済み
│   │   ├── groupshare/         # GroupShare（異なるDID間のChannel型データ共有）✅ 実装済み
│   │   └── syncdb/             # 旧同期 DB 層（廃止予定 → devicesync + groupshare）
│   ├── pkg/linkself/           # 公開 API（Client インターフェース、型定義）
│   ├── cmd/linkself-daemon/    # JSON-RPC daemon（Electron 等から子プロセスで利用）
│   └── test/integration/       # 多ノード統合テスト
├── chat-client/                # サンプルチャットアプリ（Electron + React + TypeScript）
│   ├── src/                    # main / preload / renderer（React コンポーネント・hooks）
│   ├── build/                  # ビルド成果物（daemon は core からビルド）
│   └── docs/                   # 実装計画・結合改善・テストガイド・実装変更履歴等
└── docs/                       # 全体設計・計画・多言語ドキュメント
    ├── README.ja.md, README.en.md
    ├── phase1-design.md        # Phase 1 設計（実装済み）
    ├── group-concept.md        # グループの概念
    ├── sync-db-plan.md        # DeviceSync / GroupShare 二層アーキテクチャ設計
    ├── sample-chat-app-plan.md # サンプルアプリ（チャット＋ファイル共有）計画
    ├── using-linkself-as-library.md
    ├── gitcoin-funding.md
    └── roadmap-revision-plan.md
```

---

## 3. ロードマップ（README 準拠）

| Phase | 内容 | 状態 |
|-------|------|------|
| **Phase 1** | コアロジック（DID・DHT・認証・Store-and-Forward）の定義・実装、ローカル多ノードテスト | ✅ 完了 |
| **Phase 2** | インフラモジュール整備。LinkSelf をインフラとして使う 1 本の本格サンプルアプリ（チャット＋ファイル共有）。ライブラリ利用の道筋整備 | 🔲 進行中・計画 |
| **Phase 3** | プラットフォーム展開（PC、Android、iOS） | 🔲 未着手 |

---

## 4. 現状サマリ（リポジトリ全体）

### 4.1 Core（core/）

| 項目 | 状態 | 備考 |
|------|------|------|
| internal: did, dht, auth, node, storeforward, group | ✅ | Phase 1 実装済み |
| internal: devicesync | ✅ | DeviceSync コア実装済み（MemStorage + ReplicationEngine、18テスト・91.1%カバレッジ） |
| internal: groupshare | ✅ | GroupShare コア実装済み（MemSharedStorage + GroupShareLayer、17テスト・88.5%カバレッジ） |
| internal: syncdb | ⚠️ 廃止予定 | devicesync + groupshare で置換。Phase E で削除 |
| pkg/linkself 公開 API | ✅ | Client, Config, NewClient, Start/Stop, SendMessage, Connect, SetOnMessage |
| cmd/linkself-daemon | ✅ | JSON-RPC、**pkg/linkself のみに依存**（internal 直接依存なし） |
| DHT | ✅ | 常に公開 DHT（/ipfs）。FindPeer(DIDToPeerID) で検索。UsePublicDHT は廃止 |
| 統合テスト（test/integration） | ✅ | 多ノード DHT・認証・メッセージ・Store-and-Forward 検証 |
| SendToGroup / ConnectToGroup | 🔲 計画 | 現状は SendMessage / Connect（1 対 1）。core/README に移行計画の記載あり |

### 4.2 サンプルチャットアプリ（chat-client/）

| 項目 | 状態 | 備考 |
|------|------|------|
| Electron + React + TypeScript + Vite | ✅ | 基本構造・ビルド設定済み |
| LINE ライク UI（ChatWindow, MessageList, MessageBubble, MessageInput, ContactList, PendingRequests） | ✅ | 実装済み |
| アプリ ↔ daemon 統合 | ✅ | main が core の linkself-daemon を起動、JSON-RPC で通信 |
| 起動・DID 表示 | ✅ | DID のみ表示・コピー（Listen 表示は廃止） |
| 友達追加機能 | ✅ | DID 入力・友達申請・承認/拒否・永続化（contacts.json, friend-requests.json） |
| 実際の P2P 送受信（UI から） | ✅ | 1 対 1 メッセージ送受信（Connect は DHT のみ、DID 指定） |
| 複数プラットフォーム | 🔲 後続 | 現状 Windows 優先（build:daemon が .exe 固定の可能性） |
| daemon 配置 | ✅ | core/cmd/linkself-daemon（pkg/linkself のみ依存） |

### 4.3 ドキュメント（docs/）

| 項目 | 状態 | 備考 |
|------|------|------|
| 全体説明・ロードマップ（README, docs/README.*） | ✅ | 日英あり |
| Phase 1 設計・グループ概念・sync-db 計画・サンプルアプリ計画 | ✅ | 計画・設計として記載済み |
| ライブラリ利用（using-linkself-as-library） | ✅ | 道筋のドキュメントあり |
| Gitcoin 手順（gitcoin-funding） | ✅ | 申請手順を記載 |
| CONTRIBUTING / CODE_OF_CONDUCT | 🔲 オプション | roadmap-revision-plan で言及 |

---

## 5. 参照ドキュメント（代表）

- **概要・ロードマップ**: [README.md](README.md)、[docs/README.ja.md](docs/README.ja.md)
- **Core**: [core/README.md](core/README.md)、[core/pkg/linkself/README.md](core/pkg/linkself/README.md)
- **設計・計画**: [docs/phase1-design.md](docs/phase1-design.md)、[docs/group-concept.md](docs/group-concept.md)、[docs/sample-chat-app-plan.md](docs/sample-chat-app-plan.md)、[docs/sync-db-plan.md](docs/sync-db-plan.md)、[docs/linkself-data-persistence-plan.md](docs/linkself-data-persistence-plan.md)
- **チャットアプリ**: [chat-client/README.md](chat-client/README.md)、[chat-client/docs/implementation-plan.md](chat-client/docs/implementation-plan.md)、[chat-client/docs/testing-guide.md](chat-client/docs/testing-guide.md)
- **テスト動作・現仕様**: [chat-client/docs/entry-point.md](chat-client/docs/entry-point.md)、[chat-client/docs/implementation-changes.md](chat-client/docs/implementation-changes.md)
- **友達追加・申請承認・複数インスタンス**: [chat-client/docs/friend-add-and-multi-instance.md](chat-client/docs/friend-add-and-multi-instance.md)

---

## 6. 次の作業候補（リポジトリ全体）

### DeviceSync / GroupShare 二層アーキテクチャ（Phase 2 コア）

- [x] **DeviceSync コア実装** — MemStorage + ReplicationEngine（18テスト・91.1%カバレッジ）
- [x] **GroupShare コア実装** — MemSharedStorage + GroupShareLayer（17テスト・88.5%カバレッジ）
- [x] **設計ドキュメント更新** — sync-db-plan, linkself-data-persistence-plan, sample-chat-app-plan 等を新アーキテクチャに更新
- [ ] **公開 API 拡張**（Phase C）— `pkg/linkself` に `DeviceDB()` / `GroupShare()` / `Groups()` を追加
- [ ] **Node プロトコル分離** — `/linkself/devicesync/1.0.0`, `/linkself/groupshare/1.0.0` を追加
- [ ] **daemon JSON-RPC 拡張** — `devicedb.*`, `groupshare.*`, `groups.*` メソッド追加
- [ ] **SQLite 参照実装** — DeviceStorage / SharedStorage の SQLite 実装
- [ ] **差分同期ハンドシェイク** — DeviceSync の SyncWith（high-water mark 交換 → 差分送信）
- [ ] **旧 syncdb 廃止**（Phase E）— `core/internal/syncdb/` を削除

### サンプルアプリ・クライアント

- [x] **サンプルチャットアプリ: 友達追加機能** — DID 入力・友達申請・承認/拒否・永続化は実装済み
- [ ] **チャットクライアントを DeviceDB + GroupShare に移行** — contacts/friend-requests を DeviceDB 経由に変更、メッセージ送受信を GroupShare Channel 経由に変更
- [ ] **Phase 2 本格サンプルアプリ**: ファイル共有スコープの検討・実装（sample-chat-app-plan に沿う）

### その他

- [ ] **Core API 進化**: SendToGroup / ConnectToGroup への移行（core/README 記載の計画）
- [ ] **マルチプラットフォーム**: build:daemon のバイナリ名・パスを OS 別にし、chat-client の起動パスを拡張
- [ ] **ドキュメント**: CONTRIBUTING.md / CODE_OF_CONDUCT.md の追加（任意）
- [ ] **Gitcoin**: グラントページ作成・ラウンド申請（gitcoin-funding 手順参照）

---

*最終更新: DeviceSync / GroupShare 二層アーキテクチャ実装・ドキュメント更新反映（2026-03）*
