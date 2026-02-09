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
│   │   └── syncdb/             # 同期 DB 層
│   ├── pkg/linkself/           # 公開 API（Client インターフェース、型定義）
│   ├── cmd/linkself-daemon/    # JSON-RPC daemon（Electron 等から子プロセスで利用）
│   └── test/integration/       # 多ノード統合テスト
├── chat-client/                # サンプルチャットアプリ（Electron + React + TypeScript）
│   ├── src/                    # main / preload / renderer（React コンポーネント・hooks）
│   ├── daemon/                 # ※旧 daemon（internal 直接依存、未使用）
│   ├── build/                  # ビルド成果物（daemon は core からビルド）
│   └── docs/                   # 実装計画・結合改善・テストガイド等
└── docs/                       # 全体設計・計画・多言語ドキュメント
    ├── README.ja.md, README.en.md
    ├── phase1-design.md        # Phase 1 設計（実装済み）
    ├── group-concept.md        # グループの概念
    ├── sync-db-plan.md        # 分散 DB 化計画
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
| internal: did, dht, auth, node, storeforward, group, syncdb | ✅ | Phase 1 実装済み |
| pkg/linkself 公開 API | ✅ | Client, Config, NewClient, Start/Stop, SendMessage, Connect, SetOnMessage |
| cmd/linkself-daemon | ✅ | JSON-RPC、**pkg/linkself のみに依存**（internal 直接依存なし） |
| 統合テスト（test/integration） | ✅ | 多ノード DHT・認証・メッセージ・Store-and-Forward 検証 |
| SendToGroup / ConnectToGroup | 🔲 計画 | 現状は SendMessage / Connect（1 対 1）。core/README に移行計画の記載あり |

### 4.2 サンプルチャットアプリ（chat-client/）

| 項目 | 状態 | 備考 |
|------|------|------|
| Electron + React + TypeScript + Vite | ✅ | 基本構造・ビルド設定済み |
| LINE ライク UI（ChatWindow, MessageList, MessageBubble, MessageInput, ContactList） | ✅ | モック表示まで |
| アプリ ↔ daemon 統合 | ✅ | main が core の linkself-daemon を起動、JSON-RPC で通信 |
| 起動・DID 表示 | ✅ | 統合確認済み |
| 友達追加機能 | 🔲 次に実装 | DID を入力して連絡先に追加する UI・永続化 |
| 実際の P2P 送受信（UI から） | 🔲 後続 | 複数ノード間の実メッセージ送受信は未実装 |
| 複数プラットフォーム | 🔲 後続 | 現状 Windows 優先（build:daemon が .exe 固定の可能性） |
| daemon 配置 | ✅ | core/cmd/linkself-daemon（pkg/linkself のみ依存）。旧 chat-client/daemon は削除済み |

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
- **設計・計画**: [docs/phase1-design.md](docs/phase1-design.md)、[docs/group-concept.md](docs/group-concept.md)、[docs/sample-chat-app-plan.md](docs/sample-chat-app-plan.md)、[docs/sync-db-plan.md](docs/sync-db-plan.md)
- **チャットアプリ**: [chat-client/README.md](chat-client/README.md)、[chat-client/docs/implementation-plan.md](chat-client/docs/implementation-plan.md)、[chat-client/docs/testing-guide.md](chat-client/docs/testing-guide.md)
- **友達追加・申請承認・複数インスタンス**: [chat-client/docs/friend-add-and-multi-instance.md](chat-client/docs/friend-add-and-multi-instance.md)

---

## 6. 次の作業候補（リポジトリ全体）

- [ ] **サンプルチャットアプリ: 友達追加機能** — DID を入力して連絡先に追加する UI と永続化を実装
- [ ] **Phase 2 本格サンプルアプリ**: 実 P2P 送受信を UI で完了させ、必要に応じてファイル共有スコープを検討（sample-chat-app-plan に沿う）
- [ ] **Core API 進化**: SendToGroup / ConnectToGroup への移行（core/README 記載の計画）
- [ ] **マルチプラットフォーム**: build:daemon のバイナリ名・パスを OS 別にし、chat-client の起動パスを拡張
- [ ] **ドキュメント**: CONTRIBUTING.md / CODE_OF_CONDUCT.md の追加（任意）
- [ ] **Gitcoin**: グラントページ作成・ラウンド申請（gitcoin-funding 手順参照）

---

*最終更新: リポジトリ全体を包含する形に改訂*
