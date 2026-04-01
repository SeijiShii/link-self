# トピックベース・サブスクリプションフィルタリング

**日本語**（このページ）| [English](topic-subscription-filtering.en.md)
**参照:** [データ同期設計（DeviceSync / GroupShare）](sync-db-plan.md)、[グループの概念](group-concept.md)

---

## 1. 概要

GroupShare レイヤーに**トピックベースのサブスクリプションフィルタリング**を導入する。
各デバイスは受信したいデータの範囲をトピック単位で宣言し、送信側はその宣言に基づいてデータを選択的に送信する。

### 背景・動機

- サーバーレス P2P インフラでは、全データを全端末に送信するのは帯域・ストレージの無駄
- 権限管理はアプリ層に委譲する設計思想と両立しつつ、インフラ層でデータ転送を最適化したい
- 例: 編集スタッフは 100 件の書類全体にアクセス、一般スタッフは割り当てられた書類のみ受信

### 設計原則

1. **送信側フィルタリング（Sender-side filtering）**: 帯域節約のため、送信前に不要データを除外
2. **安全なデフォルト（Safe defaults）**: サブスクリプション未登録 → 送信（データ損失防止）
3. **受信側 CanRead は安全ネット**: 既存の AccessPolicy チェックは引き続き有効
4. **アプリ層からの権限委譲と両立**: インフラはフィルタリングの仕組みだけ提供し、何をフィルタするかはアプリが決定

---

## 2. データモデル

### 2.1 SharedRecord の Topic フィールド

```go
type SharedRecord struct {
    ID        string `json:"id"`
    Channel   string `json:"channel"`
    Topic     string `json:"topic,omitempty"` // トピック（サブスクリプションフィルタリング用）
    GroupID   string `json:"group_id"`
    DID       string `json:"did"`
    Timestamp int64  `json:"timestamp"`
    Body      []byte `json:"body"`
    Deleted   bool   `json:"deleted"`
}
```

- `Topic` はオプショナル（空文字列 = トピックなし）
- アプリが任意の文字列を指定（例: ドキュメント ID `"doc-001"`）
- チャネル内でのサブフィルタリングに使用

### 2.2 SubAnnouncement

```go
type SubAnnouncement struct {
    DID     string   `json:"did"`
    Channel string   `json:"channel"`
    Topics  []string `json:"topics"`
}
```

ピア間でサブスクリプション状態を交換するためのメッセージ。

---

## 3. サブスクリプションストア

### 3.1 インターフェース

```go
type SubscriptionStore interface {
    SetSubscription(did, channel string, topics []string) error
    GetSubscription(did, channel string) ([]string, error)
    GetAllSubscriptions(did string) (map[string][]string, error)
}
```

### 3.2 実装

| 実装 | 用途 | 永続性 |
|------|------|--------|
| `MemSubscriptionStore` | リモートピアのサブスクリプション（RemoteSubs） | インメモリ（揮発） |
| `DeviceSyncSubscriptionStore` | 自身のサブスクリプション（LocalSubs） | DeviceSync 経由で同一 DID デバイス間で自動レプリケーション |

### 3.3 DeviceSyncSubscriptionStore の詳細

- DeviceSync の `ReplicationEngine` を利用して永続化
- テーブル名: `_groupshare_subs`
- レコード ID フォーマット: `{did}::{channel}`（例: `did:key:zAlice::docs`）
- Body: JSON `{"topics": ["doc-001", "doc-002"]}`
- 同一 DID の別デバイスに自動レプリケーションされる

---

## 4. フィルタリングロジック

### 4.1 トピックマッチング

```go
func TopicMatches(subscribed []string, topic string) bool
```

| subscribed | topic | 結果 | 理由 |
|------------|-------|------|------|
| `["*"]` | 任意 | `true` | ワイルドカード |
| `["doc-001", "doc-002"]` | `"doc-001"` | `true` | 完全一致 |
| `["doc-001"]` | `"doc-002"` | `false` | 不一致 |
| `[]` | 任意 | `false` | 空リスト = 全拒否 |
| `["*"]` | `""` | `true` | ワイルドカードは空トピックにもマッチ |

### 4.2 broadcast() のフィルタリングフロー

```
送信対象メンバーリスト取得
  ↓
RemoteSubs が nil → 全員に送信（フィルタリング無効）
  ↓
各メンバーについて:
  ├─ サブスクリプション未登録（nil） → 送信（安全なデフォルト）
  ├─ TopicMatches = true → 送信
  └─ TopicMatches = false → スキップ
  ↓
フィルタ後のメンバーに送信
```

**安全なデフォルト**: サブスクリプションが未登録の場合は送信する。これにより：
- サブスクリプション機能を使わないアプリでもデータが届く
- ネットワーク遅延で SubAnnouncement が未着でもデータ損失しない
- 受信側の `AccessPolicy.CanRead()` が最終的な安全ネットとして機能

---

## 5. SubAnnouncement プロトコル

### 5.1 エンベロープタイプ

```go
const TypeSubAnnounce Type = "sub_announce"
```

GroupShare データメッセージ（`TypeGroupShare`）とは別のエンベロープタイプを使用。

### 5.2 フロー

```
1. アプリが Subscribe(channel, topics) を呼び出す
2. LocalSubs に保存（DeviceSync 経由で同一 DID デバイスに複製）
3. MemberResolver でグループメンバーを取得
4. SubAnnouncement を JSON エンコード
5. TypeSubAnnounce エンベロープでラップ
6. 全メンバーにブロードキャスト
```

### 5.3 受信側処理

```
1. TypeSubAnnounce エンベロープを受信
2. HandleSubAnnouncement(senderDID, payload) を呼び出す
3. JSON デコードして SubAnnouncement を取得
4. DID 検証: senderDID == announcement.DID（不一致は拒否 = スプーフィング防止）
5. RemoteSubs に保存
```

---

## 6. 公開 API（pkg/linkself）

### 6.1 GroupShareAPI インターフェース

```go
type GroupShareAPI interface {
    RegisterChannel(name, groupID string, opts ...ChannelOption) error
    Subscribe(channel string, topics []string) error
    Put(ctx context.Context, channel, topic, recordID string, body []byte) error
    Get(ctx context.Context, channel, recordID string) (*SharedRecord, error)
    Delete(ctx context.Context, channel, topic, recordID string) error
    List(ctx context.Context, channel string) ([]*SharedRecord, error)
}
```

- `Put` / `Delete` に `topic` パラメータを追加
- `Subscribe` メソッドを新規追加
- 完全な GroupShareAPI（Dump/Restore/Purge 含む）は [dump-restore-retention.md §4.1](dump-restore-retention.md) を参照

### 6.2 デーモン RPC

| メソッド | パラメータ |
|----------|------------|
| `groupshare.put` | `channel`, `topic`, `record_id`, `body` |
| `groupshare.delete` | `channel`, `topic`, `record_id` |
| `groupshare.subscribe` | `channel`, `topics` |

---

## 7. ワイヤリング（client.go）

```
GroupShareLayer
├── LocalSubs  = DeviceSyncSubscriptionStore(dsEngine)  ← DeviceSync 経由で永続化
├── RemoteSubs = MemSubscriptionStore()                  ← インメモリ
└── SendSubAnnounce = envelope.Wrap(TypeSubAnnounce) → node.SendToGroup

MessageRouter
├── OnGroupShare  → gsLayer.HandleIncoming
└── OnSubAnnounce → gsLayer.HandleSubAnnouncement
```

---

## 8. テストカバレッジ

### ユニットテスト

| テスト対象 | テスト数 | カバー範囲 |
|------------|----------|------------|
| `TopicMatches` | 8 | ワイルドカード、完全一致、不一致、空リスト、空トピック |
| `MemSubscriptionStore` | 4 | Set/Get、上書き、購読解除、GetAll |
| `DeviceSyncSubscriptionStore` | 6 | Set/Get、上書き、購読解除、GetAll、マルチ DID、レプリケーション同期 |
| `GroupShareLayer` フィルタリング | 4 | サブスクリプションフィルタ、RemoteSubs nil、空サブスクリプション除外、Delete フィルタ |
| `Subscribe` | 3 | 正常系、LocalSubs nil、LocalSubs 保存確認 |
| `HandleSubAnnouncement` | 3 | 正常系、DID 不一致拒否、RemoteSubs nil |

### 統合テスト

- `groupshare_integration_test.go`: Put 呼び出しに空トピックを渡して既存テストとの互換性を確認

### カバレッジ: **89.2%**

---

## 9. 利用例：ドキュメント管理アプリ

```
チャネル: "documents"
トピック: ドキュメント ID（例: "doc-001", "doc-002", ...）

【編集スタッフ】
  Subscribe("documents", ["*"])  → 全ドキュメントを受信

【一般スタッフ A（doc-001, doc-003 担当）】
  Subscribe("documents", ["doc-001", "doc-003"])  → 担当分のみ受信

【一般スタッフ B（doc-002 担当）】
  Subscribe("documents", ["doc-002"])  → doc-002 のみ受信
```

送信側は各メンバーのサブスクリプションを参照し、必要なデータのみを送信する。
サブスクリプション未登録のメンバーには安全のため全データを送信する。

---

## 10. 再接続時の SubAnnouncement ハンドシェイク

> **追加（2026-04）**

RemoteSubs はインメモリ（揮発）であるため、ピアが切断→再接続すると送信側がそのピアのサブスクリプション情報を失う。これを防ぐため、接続確立時に SubAnnouncement を自動交換するハンドシェイクを追加する。

### フロー

```
1. ピア A とピア B が接続を確立（認証完了後）
2. 両者が自身の全 SubAnnouncement を相手に送信
3. 受信側は RemoteSubs を更新
```

- 認証完了後のコールバックに組み込む（既存の Store-and-Forward フラッシュと同様のタイミング）
- LocalSubs の全エントリを SubAnnouncement として送信する
- 既存の SubAnnouncement 処理（§5.3）をそのまま再利用

---

## 11. SQL インターフェースへの移行（予定）

> **追加（2026-04）:** [データ同期コンセプト](data-sync-concept.md) の決定事項を反映。

最終的にアプリ向け公開 API は SQL クエリインターフェースに移行する。現行の Channel/Topic ベースのフィルタリングは以下のようにマッピングされる。

| 現行概念 | SQL モデルでの対応 |
|---------|------------------|
| Channel | テーブル |
| Topic | アプリが定義するカラム値（WHERE 句でフィルタ） |
| `Subscribe(channel, topics)` | `Subscribe(table, filter条件)` |

現行の Put/Get API と Channel/Topic の仕組みは内部実装として残る。
