# Response: home-visit-suite プロポーザルへの回答

**From:** LinkSelf
**To:** home-visit-suite
**Date:** 2026-03-31
**Re:** [Proposal: LinkSelf ストレージ実装の外部注入](proposal-from-home-visit-suite.md)

---

## 前提の明確化: LinkSelf のストレージは同期トランスポート層である

プロポーザルの検討にあたり、まず LinkSelf のストレージ層の位置づけを明確にする。

### DeviceDB / GroupShare は「同期トランスポート」であり「アプリの汎用DB」ではない

LinkSelf のストレージ（`device_records`, `shared_records`, `change_log`）は、**デバイス間・ユーザー間のデータ同期を実現するためのトランスポート層**である。

```
┌─────────────────────────────────────────────────────────┐
│ アプリ層                                                 │
│  アプリ独自の SQLite DB（リッチなクエリ、JOIN、検索）        │
│  ← アプリが自由にスキーマ設計・マイグレーション管理          │
├─────────────────────────────────────────────────────────┤
│ LinkSelf 同期トランスポート層                              │
│  DeviceDB: table + id + body (BLOB) → 同一DID全デバイスに複製│
│  GroupShare: channel + topic + id + body → グループメンバーに配信│
│  ← スキーマレス。body の中身は LinkSelf にとって不透明       │
└─────────────────────────────────────────────────────────┘
```

**LinkSelf のストレージが提供するもの:**
- 論理名前空間（`table`, `channel`）による分類
- レコード単位の CRUD
- last-write-wins による自動競合解決
- ChangeLog ベースの差分同期

**LinkSelf のストレージが提供しないもの:**
- body 内フィールドによる WHERE 条件付き検索
- テーブル間の JOIN
- アプリ固有のスキーマ制約（型、NOT NULL、UNIQUE 等）
- アプリ固有のインデックス

**推奨されるアプリ側の設計:**

アプリが複雑なクエリを必要とする場合（home-visit-suite のような業務アプリでは通常必要）、アプリ側に独自の SQLite DB を持ち、LinkSelf の同期データを反映する構成が適切である:

```
DeviceDB.Put("visit_records", id, body)
    ↓ 同一DIDの全デバイスに自動同期
    ↓ 受信側
アプリのコールバックで受信 → アプリ側 SQLite に INSERT/UPDATE
    ↓
アプリ側 SQLite に対してリッチなクエリ実行
  SELECT * FROM visit_records WHERE patient_id = ? AND date > ?
```

この構成では:
- **同期の責務**は LinkSelf が担う（アプリは同期ロジックを書かない）
- **クエリの責務**はアプリが担う（アプリは自由にスキーマ設計できる）
- **データの所有権**は LinkSelf の DID 空間にある（永続化方針に合致）

> この位置づけは [sync-db-plan.md](../../docs/sync-db-plan.md) および [linkself-data-persistence-plan.md](../../docs/linkself-data-persistence-plan.md) に反映する予定。

---

## プロポーザルへの回答

### 受け入れ: ストレージバックエンドの選択を Config で指定可能にする

**プロポーザルの要旨:** MemStorage がハードコードされており、永続ストレージを使えない。

**これは正当な要求であり、対応する。** ただし、個別インターフェースの注入ではなく、バックエンド全体を選択する形で実現する。

```go
// 受け入れる形
client.Start(ctx, linkself.Config{
    IdentityPath:   "~/.home-visit-suite/identity.json",
    StorageBackend: linkself.SQLiteBackend("~/.home-visit-suite/linkself.db"),
})

// nil（デフォルト）なら従来通り MemStorage を使用（後方互換）
```

**理由:**
- 既存の `core/internal/storage/sqlite/` に全5インターフェースの実装が揃っている
- LinkSelf 内部でストレージの整合性を管理でき、レプリケーションの不整合リスクがない
- アプリはストレージの「場所」を指定するだけで、内部構造を知る必要がない
- 将来的に DID 空間パスの自動解決と組み合わせ可能

### 受け入れ: SQLite 実装の品質保証と公開

**プロポーザル末尾の選択肢:** 「LinkSelf が品質保証した SQLite3 実装をそのまま使える」

**これを正式な方針とする。** 現在 `core/internal/storage/sqlite/` にある実装を、`StorageBackend` 経由で外部から利用可能にする。アプリが独自にストレージを実装する必要はない。

### 拒絶: 個別ストレージインターフェースの pkg/linkself への公開

**プロポーザル §1:** DeviceStorage, SharedStorage, SubscriptionStore, GroupStore, RecordStorage を公開 API に追加する。

**拒絶する。理由:**

1. **抽象レベルの破壊。** 現在の公開 API（`DeviceDB`, `GroupShareAPI`, `GroupAPI`）は意図的に高レベルな抽象を提供している。`AppendChange()`, `ChangesSince()`, `LatestSeq()` などの ChangeLog 操作は ReplicationEngine の内部メカニズムであり、外部に公開するとレプリケーションの整合性をアプリが壊すリスクがある。

2. **不要。** アプリが必要とするのは「永続ストレージを使いたい」であり、「ストレージ実装を自作したい」ではない。`StorageBackend` の指定で目的は達成される。

3. **GroupStore の直接公開は危険。** 現在の `GroupAPI` はドメインロジック（オーナー管理、脱退ルール、自動昇格等）を制御している。`GroupStore` を直接公開すると、このロジックをバイパスして不整合なグループ状態を作れてしまう。

### 拒絶: Config への個別ストレージ注入フィールド

**プロポーザル §2:** Config に DeviceStorage, SharedStorage 等のフィールドを追加する。

**拒絶する。理由:**

1. **部分注入の危険。** 5つのストレージのうち一部だけ注入し、残りが MemStorage になると、データの永続化状態が不整合になる（例: GroupStore は永続化されているが SharedStorage はメモリのみ → 再起動でグループの共有データが消失）。

2. **LinkSelf のデータ管理責任の喪失。** アプリがストレージ実体を外部で作成・管理すると、DID 空間によるデータ分離の仕組みが機能しなくなる。複数アプリが同じ DID のデータを共有できなくなる。

3. **`StorageBackend` で代替可能。** 単一のバックエンド指定で、内部の全ストレージが一貫して切り替わる方が安全。

### 拒絶: アプリ側での独自ストレージ実装

**プロポーザル §1 の暗黙の前提:** アプリが各インターフェースを独自に（例: アプリ側の SQLite で）実装する。

**拒絶する。理由:**

1. **トランスポート層のストレージはインフラの内部詳細。** ChangeLog のシーケンス管理、last-write-wins のタイムスタンプ比較、SharedRecord のライフサイクル管理は LinkSelf の同期プロトコルと密結合しており、外部実装がこれらの不変条件を正しく維持する保証がない。

2. **アプリが必要とするリッチなクエリは、アプリ側 DB で対応すべき。** 前述の「同期トランスポート + アプリ側DB」構成が適切。

---

## 実装計画

| 項目 | 対応 |
|------|------|
| `Config.StorageBackend` フィールド追加 | 新規実装 |
| `linkself.SQLiteBackend(path)` ファクトリ関数 | `internal/storage/sqlite` をラップ |
| `linkself.MemoryBackend()` 明示的指定（任意） | 既存 MemStorage をラップ |
| nil 時のデフォルト動作 | MemStorage（後方互換） |
| `client.go` の `Start()` 修正 | バックエンド有無で分岐 |
| 個別インターフェースの pkg 公開 | **しない** |

### StorageBackend インターフェース案

```go
// pkg/linkself/types.go に追加

// StorageBackend provides all storage implementations as a unit.
// Use SQLiteBackend() or MemoryBackend() to create one.
// Applications should not implement this interface directly.
type StorageBackend interface {
    // internal use only — not exported as individual accessors
    internal()
}
```

アプリからは中身を見ることも差し替えることもできない不透明なオブジェクトとして渡す。

---

## まとめ

| プロポーザルの要求 | 判定 | 理由 |
|---|---|---|
| 永続ストレージを使いたい | **受け入れ** | `Config.StorageBackend` で対応 |
| LinkSelf 内蔵 SQLite を使いたい | **受け入れ** | `SQLiteBackend(path)` で公開 |
| ストレージインターフェースを公開したい | **拒絶** | 同期トランスポートの内部詳細。公開不要 |
| アプリ側でストレージ実装を注入したい | **拒絶** | レプリケーション整合性リスク。不要 |
| Config に個別ストレージフィールドを追加 | **拒絶** | 部分注入の危険。`StorageBackend` で代替 |

home-visit-suite が実際に必要としているのは「再起動してもデータが消えない」ことであり、それは `StorageBackend` の指定だけで実現できる。アプリ固有のリッチなクエリ要求は、LinkSelf の同期データをアプリ側 DB に反映する構成で対応する。
