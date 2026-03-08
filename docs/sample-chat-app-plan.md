# サンプルアプリ計画（チャット＋ファイル共有）

**日本語**（このページ）| [English](sample-chat-app-plan.en.md)  
**ステータス:** 実装保留（計画のみ docs に保存）  
**概要:** **LinkSelf をインフラとして使用**する 1 本の本格サンプルアプリの設計。グループ/1対1 チャットと P2P ファイル共有を備える。グループ情報・連絡先・同期メタ等は **LinkSelf 側の DB** に保存され、アプリは LinkSelf の API 経由で参照・操作する。
**参照:** [グループの概念](group-concept.md)、[Phase 1 設計](phase1-design.md)、[データ同期設計（DeviceSync / GroupShare）](sync-db-plan.md)、[ロードマップ](README.ja.md#roadmap)

> **注意（2026-03）:** データ同期は **DeviceSync / GroupShare 二層アーキテクチャ** に移行。旧 SyncLayer は DeviceDB + GroupShare に置き換わる。

---

## 役割とデータ

- **1 つのアプリ、LinkSelf をインフラとして:** サンプルアプリは **1 本のアプリ** であり、LinkSelf をライブラリとして組み込み、P2P インフラとして利用する。他アプリから LinkSelf を利用する **第一の利用例** となる。
- **データは LinkSelf 側の DB:** **グループ情報・連絡先・同期メタ** などは **LinkSelf のストア（DB）** に保存される。アプリは LinkSelf の API（例: group.Store、SyncLayer）経由で読み書きする。アプリはグループメンバーシップを独自に持たず、LinkSelf に委ねる。

---

## 結論

- **本格サンプルアプリ（チャット＋ファイル共有）**: 実装可能。1 本のアプリがコアの Node API（`New`, `Start`, `SetOnMessage`, `SendToGroup`, `ConnectToGroup`）および LinkSelf が提供する SyncLayer / RecordStorage を利用する。1 対 1 は **2 人グループ** のケースとして扱う（[グループの概念](group-concept.md) 参照）。
- **1台のPCで複数ノード**: 可能。**1プロセス＝1ノード** とし、同じPCで同じバイナリを複数起動（複数ターミナル）すれば、複数ノードを再現できる。統合テスト [core/test/integration/integration_test.go](../core/test/integration/integration_test.go) がすでに同一ホスト上で複数ノード（別ポート）を立てて DHT・認証・メッセージを検証している。

---

## アーキテクチャ

```mermaid
flowchart LR
  subgraph terminal1 [ターミナル1]
    P1[プロセス1]
    N1[Node A]
    P1 --> N1
  end
  subgraph terminal2 [ターミナル2]
    P2[プロセス2]
    N2[Node B]
    P2 --> N2
  end
  N1 <- ->|"DHT / TCP"| N2
```

- 各プロセスで `node.New` → `node.Start` で1ノードを起動。
- 先に起動したノードのアドレス（Listen アドレス + PeerID）を、2台目以降の `BootstrapPeers` に渡す（統合テストと同様）。DHT は常に公開 DHT（/ipfs）を使用。
- 同一マシンでは `ListenAddrs: ["/ip4/127.0.0.1/tcp/0"]` でポート自動割当にすれば、複数ノードが衝突しない。

---

## 実装方針

### 配置

- **Phase 1 設計**では `core/cmd/linkself/` は「サンプルアプリ用エントリ（チャット＋ファイル共有、未実装）」とされている。サンプルアプリのエントリポイントを **`core/cmd/linkself/`** に置く（1 バイナリで LinkSelf を組み込み、チャット＋ファイル共有を提供する形）。

### 機能（最小）

| 項目 | 内容 |
|------|------|
| 起動 | 1プロセスで1ノード起動。`ListenAddrs: /ip4/127.0.0.1/tcp/0`（オプションでポート指定可）。 |
| identity | 初回は `did.Generate()` で生成し、ローカルに保存（例: 作業ディレクトリの `identity.json` や libp2p の鍵形式）。2回目以降は読み込み。 |
| bootstrap | フラグで「1台目のアドレス」を渡す（例: `--bootstrap /ip4/127.0.0.1/tcp/xxxx/p2p/<PeerID>`）。2台目以降はこれで DHT に参加。 |
| 連絡先 | Phase 1 の「保存のみ」に合わせ、DID をローカルファイルまたはメモリのリストで保持。 |
| 受信 | `SetOnMessage` で受け取った payload を標準出力に表示（プレーン文字列または簡易 JSON）。 |
| 送信 | 対話的に「グループ（メンバー DID のリスト）」と「本文」を入力 → `ConnectToGroup(ctx, memberDIDs)` で認証・フラッシュしてから `SendToGroup(ctx, memberDIDs, []byte(text))`。2 人チャットの場合は 2 人グループ `[myDID, peerDID]` として扱う。 |
| 終了 | `Close()` を呼んでからプロセス終了。 |

### ファイル共有（スコープ）

- **スコープ:** P2P ファイル送受信またはグループ内でのファイル共有（共有フォルダ的な同期）。ファイルメタデータ（名前・サイズ・ハッシュ・チャンク ID）は SyncLayer で共有し、実体（チャンク）は別プロトコル（例: Node のストリーム）で送る。詳細（チャンクサイズ・再送・部分取得）は実装時に決定する。
- **追加する機能:** ファイル送信、受信、一覧、削除、オフライン時のチャンクキュー。

### メッセージ形式

- コアは **任意の `[]byte`** を送受信する。サンプルでは「UTF-8 テキスト1行」や、必要なら `{"from":"<DID>","text":"..."}` のような最小 JSON のどちらかに統一する。

### 運用イメージ（1台PCで2ノード・2人グループ）

1. **ターミナル1**: `go run ./cmd/linkself`（またはビルドした `linkself`）  
   → 「My DID: did:key:...」を表示。daemon の Start 結果では Listen アドレスも返るが、チャットクライアントでは DID のみ表示する。
2. **ターミナル2**: `linkself --bootstrap /ip4/127.0.0.1/tcp/4001/p2p/<PeerID>`  
   → 2台目が DHT に参加。ターミナル1の DID を「連絡先」に追加し、**2人グループ**（自分と相手の DID のリスト）を登録して `connectToGroup` してから `sendToGroup`。
3. 両方で「相手 DID を登録 → 2人グループとして connectToGroup → sendToGroup」すれば双方向チャットになる。Store-and-Forward は既にコアが担当するため、オフライン送信もそのまま利用可能。

---

## 実装タスク（案）

1. **`core/cmd/linkself/main.go` の追加**
   - フラグ: `--listen`, `--bootstrap`, `--identity`（identity ファイルパス）。
   - identity の生成・保存・読み込み（libp2p の鍵シリアライズまたは did.Identity を保持する簡易形式）。
   - `node.Config` に `ListenAddrs` と `BootstrapPeers`（`--bootstrap` から `peer.AddrInfo` を組み立て）を渡して `node.New` → `Start`。
   - 起動時に「My DID」を表示（Listen アドレスは daemon 結果に含まれるが、クライアント UI では DID のみ表示）。

2. **対話ループ**
   - 標準入力からコマンドを読む（例: `add <DID>`, `group <DID>...` でグループ登録, `connectToGroup`, `sendToGroup <text>`, `list`, `quit`）。
   - `SetOnMessage`: 受信した payload を「From <DID>: <text>」のように表示。
   - `add`: 連絡先リストに DID を追加（メモリ＋任意でファイル保存）。
   - `group`: 現在のグループ（メンバー DID のリスト）を登録。2人チャットの場合は自分と相手の 2 DID を指定。
   - `connectToGroup`: `node.ConnectToGroup(ctx, memberDIDs)` を呼び、認証・キューフラッシュを行う。
   - `sendToGroup`: `node.SendToGroup(ctx, memberDIDs, []byte(text))`。
   - `quit`: `node.Close()` して終了。

3. **README / ドキュメント**
   - `core/README.md` または `docs/` に「サンプルチャットの動かし方」を追記。
   - 1台PCで2ターミナルを起動し、bootstrap と DID を交換してチャットする手順を記載。

---

## 注意点・依存

- **internal 参照**: `cmd/linkself` は `core` モジュール内なので、`internal/did`, `internal/node`, `internal/dht` 等をそのまま import できる。
- **DHT の安定化**: 統合テストと同様、2台目が DHT で 1台目を見つけられるまでに数秒かかることがある。必要なら「connect 失敗時は数秒待ってリトライ」を入れるとよい。
- **鍵の永続化**: libp2p の `crypto.MarshalPrivateKey` / `crypto.UnmarshalPrivateKey` と `did.FromPrivKey` を使えば、既存の did パッケージと整合する。

---

## 現実の実装を想定した実用性

このサンプルのつくりは、**本番クライアント（モバイル／デスクトップ）がコアをどう使うか**と整合しており、実際的な土台になる。

### そのまま本番に持ち込める点

| 観点 | サンプル | 本番想定 | 評価 |
|------|----------|----------|------|
| **1インスタンス＝1ノード** | 1プロセス＝1ノード | 1アプリ（1デバイス）＝1ノード | 同じモデル。本番も「起動時に Node を1つ作る」でよい。 |
| **コアの使い方** | `New` → `Start` → `SetOnMessage` → `ConnectToGroup` / `SendToGroup` | 同上 | API の呼び出し順・責務は本番と同じ。gomobile/FFI でも同じフロー。1対1は2人グループのケース。 |
| **identity** | 初回生成 → ファイル保存 → 再起動時に読み込み | 初回生成 → Keychain/Keystore 等で永続化 → 起動時に復元 | 「生成→永続化→復元」の流れは同じ。永続化先がファイルかキーチェーンの違いだけ。 |
| **連絡先** | DID のリスト（メモリ＋任意でファイル） | 連絡先 DB（DID + 表示名等） | 「DID をキーにした連絡先管理」という概念は同じ。スキーマと UI が増えるだけ。 |
| **送受信** | `SendToGroup(memberDIDs, payload)` / `SetOnMessage(peerDID, payload)` | 同上＋UI 表示・通知 | ペイロード形式（例: JSON）を決めれば、本番でも同じ API で拡張可能。1対1は2人グループ。 |
| **Store-and-Forward** | コア任せ（オフライン送信もそのまま） | 同上 | 本番でも追加実装不要。 |

つまり、**ノードのライフサイクル・identity・連絡先・メッセージ API の使い方は本番と同じ**にできる。

### サンプルと本番で差が出る部分（意図した差）

| 項目 | サンプル | 本番で想定される形 | 対応のしやすさ |
|------|----------|--------------------|----------------|
| **UI** | CLI（コマンド入力） | GUI（チャット画面・連絡先一覧） | コアは UI に依存しない。GUI は「同じ API を別の入力で叩く」層として追加すればよい。 |
| **bootstrap** | 起動引数で 1 台目の multiaddr を渡す | 固定 bootstrap ノード／mDNS（Phase 2）／招待リンク（DID＋アドレス）など | 「DHT に参加するための初期接続先」という役割は同じ。本番では `BootstrapPeers` の**取得元**が変わる（QR・招待URL・設定サーバ等）。 |
| **identity の保存** | プレーンファイル | プラットフォームの安全な保存（Keychain 等） | サンプルでは「パスを渡して load/save」にしておけば、本番でその実装を差し替えるだけでよい。 |
| **連絡先の永続化** | 簡易ファイル or メモリのみ | DB やアプリのストレージ | サンプルで「DID リストの保存」を抽象化（例: インターフェース）しておくと、本番で実装を差し替えやすい。 |

これらは**プレゼンテーション層やインフラの差**であり、**Node の組み立て方や ConnectToGroup/SendToGroup の使い方は変えない**設計にできる。

### 実装時の推奨（現実実装を見据えるなら）

- **identity**: 読み書きを「パス → バイト列」の関数に分離し、後から「ファイル」「Keychain 経由」などに差し替え可能にする。
- **連絡先**: 「DID の追加・一覧・取得」を小さなインターフェース or 型にまとめ、最初はメモリ＋オプションでファイル実装にすると、本番で DB に差し替えやすい。
- **bootstrap**: `BootstrapPeers` を「起動時引数」に限定せず、設定や招待データから組み立てる関数を用意しておくと、本番で QR／招待リンク対応にしやすい。

以上から、**このサンプルは「現実の実装で使うコアの使い方」をそのまま示す実際的な土台**になっており、CLI と手動 bootstrap は「本番では別手段に置き換える部分」として明示しておけば、実装の見通しがよい。

---

## まとめ

- **1 本の本格サンプルアプリ**（チャット＋ファイル共有）が **LinkSelf をインフラとして使用**する。グループ情報・連絡先・同期メタは **LinkSelf 側の DB** に保存され、アプリは LinkSelf の API 経由で利用する。
- 送受信は **グループ単位**（SendToGroup / ConnectToGroup）とし、1対1は 2 人グループのケースとして扱う。
- **1台のPCで複数ノード** は、同じバイナリを複数プロセス（複数ターミナル）で起動し、bootstrap でつなぐ形で再現できる。
- 実装範囲: 1 エントリポイント + チャット + ファイル共有 + identity 永続化 + グループ管理（LinkSelf 経由）+ 簡単なドキュメント。GUI（Web / デスクトップ / TUI）は実装時に決定する。

## 関連ドキュメント

- [グループの概念](group-concept.md): グループ・オーナー・脱退・権限の扱い。
- [Phase 1 設計](phase1-design.md): コアのスコープと API 方針。
- [分散ネットワーク DB 化計画](sync-db-plan.md): アプリからネットワークを DB として扱う設計。
