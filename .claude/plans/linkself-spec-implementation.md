# Implementation Plan: LinkSelf Spec-to-Code Alignment

## Overview

data-sync-concept.md の決定事項（§14）と現行コードベースのギャップを埋める実装計画。
6 フェーズ構成で、各フェーズは独立にテスト・コミット可能。

---

## Current State Summary

**実装済み:**
- DeviceSync / GroupShare 二層アーキテクチャ（internal パッケージ + テスト）
- 公開 API: `Client.DeviceDB()` / `GroupShare()` / `Groups()`
- SQLite3 ストレージバックエンド（WAL モード）
- Group パッケージ（Owners フィールドあり）
- JSON-RPC daemon（全 API 対応）
- Topic サブスクリプション + Retention + Dump/Restore
- Envelope ベースメッセージルーティング

**未実装（仕様で決定済み）:**
- Suite / Network 2層概念
- ロール DAG（オーナー廃止）
- SQL クエリインターフェース
- ストレージパス自動決定
- API リネーム（MyDB / SharedDB / NetworkAPI）
- SubAnnouncement 再接続ハンドシェイク
- 差分同期時 Retention 情報伝達
- MyDB Dump/Restore
- DeviceStorage.ListTables()
- ネットワーク最低 1 人

---

## Phase 1: API Rename + MyDB Dump/Restore

**目的:** 公開 API の名称を仕様に合わせる。MyDB の Dump/Restore を追加。
**依存:** なし（最初に実行可能）
**方針:** 内部パッケージ名（devicesync, groupshare）は変更しない。公開 API 層のみ。破壊的変更を許容し、旧名の後方互換は残さない。

### Step 1.1: DeviceDB → MyDB リネーム

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | `DeviceDB` interface → `MyDB` に rename。旧名は削除 |
| `pkg/linkself/client.go` | Modify | `DeviceDB()` → `MyDB()` に rename。旧メソッド削除 |
| `pkg/linkself/api.go` | Modify | 内部 `deviceDB` struct → `myDB` に rename |
| `pkg/linkself/api_test.go` | Modify | テスト内の `DeviceDB` 参照を `MyDB` に置換 |
| `pkg/linkself/client_test.go` | Modify | 同上 |

### Step 1.2: GroupShareAPI → SharedDB リネーム

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | `GroupShareAPI` interface → `SharedDB` に rename。旧名削除 |
| `pkg/linkself/client.go` | Modify | `GroupShare()` → `SharedDB()` に rename。旧メソッド削除 |

### Step 1.3: GroupAPI → NetworkAPI リネーム

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | `GroupAPI` interface → `NetworkAPI` に rename。旧名削除 |
| `pkg/linkself/client.go` | Modify | `Groups()` → `Network()` に rename。旧メソッド削除 |

### Step 1.4: DeviceStorage.ListTables() 追加

| File | Operation | Description |
|------|-----------|-------------|
| `internal/devicesync/types.go` | Modify | `DeviceStorage` interface に `ListTables(ctx) ([]string, error)` 追加 |
| `internal/devicesync/mem_storage.go` | Modify | `MemStorage.ListTables` 実装 |
| `internal/devicesync/mem_storage_test.go` | Modify | ListTables テスト追加 |
| `internal/storage/sqlite/device_storage.go` | Modify | SQLite 版 ListTables 実装 |
| `internal/storage/sqlite/sqlite_test.go` | Modify | SQLite ListTables テスト追加 |

### Step 1.5: MyDB Dump/Restore

| File | Operation | Description |
|------|-----------|-------------|
| `internal/devicesync/replication.go` | Modify | `ReplicationEngine.Dump(ctx)` / `Restore(ctx, records)` 追加 |
| `internal/devicesync/replication_test.go` | Modify | Dump/Restore テスト追加 |
| `pkg/linkself/types.go` | Modify | `MyDB` interface に `Dump` / `Restore` 追加 |
| `pkg/linkself/api.go` | Modify | MyDB Dump/Restore の公開 API 実装 |
| `pkg/linkself/api_test.go` | Modify | テスト追加 |

### Step 1.6: daemon RPC 更新

| File | Operation | Description |
|------|-----------|-------------|
| `cmd/linkself-daemon/main.go` | Modify | `devicedb.*` → `mydb.*`、`groupshare.*` → `shareddb.*`、`groups.*` → `network.*` に rename。旧名削除 |
| `cmd/linkself-daemon/main.go` | Modify | `mydb.dump` / `mydb.restore` 追加 |

### Tests
```bash
go test ./pkg/linkself/ -timeout 120s
go test ./internal/devicesync/... -v
go test ./internal/storage/sqlite/... -v
go test ./cmd/linkself-daemon/ -v
```

---

## Phase 2: Network Concept (Group → Network Migration)

**目的:** Group パッケージを Network パッケージで置換。ネットワーク最低 1 人。ロール DAG の基盤。
**依存:** Phase 1 完了
**方針:** internal/group は削除し internal/network で置換。Owners フィールド廃止。破壊的変更。

### Step 2.1: ロール DAG パッケージ作成

| File | Operation | Description |
|------|-----------|-------------|
| `internal/role/role.go` | Create | `RoleDef`, `RoleDefs`, `DAG` 型。`HasRole(memberRole, required)` で包含判定 |
| `internal/role/role_test.go` | Create | DAG 構築、包含判定、循環参照検出テスト |

```go
// pseudo-code
type RoleDef struct { Includes []string }
type RoleDefs map[string]RoleDef
type DAG struct { defs RoleDefs; ancestors map[string]map[string]bool }

func NewDAG(defs RoleDefs) (*DAG, error) // 循環参照チェック
func (d *DAG) HasRole(memberRole, requiredRole string) bool
```

### Step 2.2: Network パッケージ作成

| File | Operation | Description |
|------|-----------|-------------|
| `internal/network/types.go` | Create | `Network` 型（ID, SuiteID, Members, MemberRoles）。最低 1 人 |
| `internal/network/store.go` | Create | `Store` interface（CRUD + ListBySuite + ListForMember） |
| `internal/network/memstore.go` | Create | インメモリ Store 実装 |
| `internal/network/service.go` | Create | `Service`（Create, Join, Leave, SetMemberRole, Kick）。権限はロール DAG ベース |
| `internal/network/service_test.go` | Create | ネットワーク CRUD、ロール権限、最低 1 人テスト |
| `internal/network/store_test.go` | Create | Store 単体テスト |

```go
// pseudo-code
type Network struct {
    ID          string
    SuiteID     string
    Members     []string
    MemberRoles map[string]string // DID → role name
}

type Service struct { store Store; dag *role.DAG; adminRole string }

func (s *Service) Create(ctx, suiteID, creatorDID string) (string, error) // min 1 member
func (s *Service) AddMember(ctx, networkID, requesterDID, memberDID, role string) error
func (s *Service) Leave(ctx, networkID, memberDID string) error
func (s *Service) Kick(ctx, networkID, requesterDID, targetDID string) error
func (s *Service) SetMemberRole(ctx, networkID, requesterDID, targetDID, role string) error
```

### Step 2.3: Group パッケージ削除 + 参照切り替え

| File | Operation | Description |
|------|-----------|-------------|
| `internal/group/` | Delete | group パッケージ全体を削除（group.go, store.go, memstore.go, テスト） |
| `internal/groupshare/layer.go` | Modify | `MemberResolver` の実装を network.Store ベースに切り替え |
| `internal/storage/sqlite/group_store.go` | Delete | SQLite group store 削除 |
| `internal/storage/sqlite/migrate.go` | Modify | groups テーブル → networks + member_roles テーブルに置換 |
| `pkg/linkself/backend.go` | Modify | backendStorages から group.Store を削除、network.Store に置換 |
| `test/integration/syncdb_test.go` | Modify | group.Store 参照を network.Store に置換 |
| `test/integration/groupshare_integration_test.go` | Modify | 同上 |

### Step 2.4: 公開 API の NetworkAPI 拡張

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | `NetworkAPI` に `Create`, `List`, `Select`, `SetMemberRole`, `GetMemberRole` 追加 |
| `pkg/linkself/api.go` | Modify | NetworkAPI 実装を network パッケージに接続 |

### Step 2.5: Config に SuiteID + Roles 追加

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | `Config` に `SuiteID string` と `Roles RoleDefs` 追加 |
| `pkg/linkself/client.go` | Modify | Start 時に DAG 構築、NetworkAPI にロール DAG 注入 |

### Step 2.6: SQLite Network Store

| File | Operation | Description |
|------|-----------|-------------|
| `internal/storage/sqlite/network_store.go` | Create | SQLite 版 network.Store 実装 |
| `internal/storage/sqlite/migrate.go` | Modify | networks + member_roles テーブルのマイグレーション追加 |

### Tests
```bash
go test ./internal/role/... -v
go test ./internal/network/... -v
go test ./pkg/linkself/ -timeout 120s
```

---

## Phase 3: Storage Auto-Placement (Suite/Network Directory Structure)

**目的:** ストレージパスを LinkSelf が自動決定。`Config.StorageBackend` を非推奨化。
**依存:** Phase 2 完了

### Step 3.1: データルート決定ロジック

| File | Operation | Description |
|------|-----------|-------------|
| `internal/dataroot/dataroot.go` | Create | プラットフォーム別データルート取得。`LINKSELF_DATA_DIR` 環境変数オーバーライド |
| `internal/dataroot/dataroot_test.go` | Create | テスト |

```go
func DefaultRoot() (string, error) // OS 別: XDG, AppData, ~/Library/...
func DIDDir(root, did string) string // root/<encoded-DID>/
func SuiteDir(root, did, suiteID string) string
func NetworkDir(root, did, suiteID, instanceID string) string
func EncodeDID(did string) string // ファイルシステム安全な変換
```

### Step 3.2: Config 変更

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | `StorageBackend` フィールド削除。`DataDir string` 追加（オプション、デフォルトは自動決定） |
| `pkg/linkself/client.go` | Modify | Start 時に dataroot でパス決定 → SQLite を自動オープン |
| `pkg/linkself/backend.go` | Delete | StorageBackend 抽象化は不要に（SQLite 固定） |
| `pkg/linkself/backend_test.go` | Delete | 同上 |

### Step 3.3: DID 一覧 API

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | `Client` に `ListDIDs() ([]string, error)` 追加 |
| `pkg/linkself/client.go` | Modify | データルート下の DID ディレクトリを列挙 |

### Step 3.4: daemon 対応

| File | Operation | Description |
|------|-----------|-------------|
| `cmd/linkself-daemon/main.go` | Modify | `start` パラメータに `suiteID` 追加。`storageBackend` パラメータ削除 |

### Tests
```bash
go test ./internal/dataroot/... -v
go test ./pkg/linkself/ -timeout 120s
```

---

## Phase 4: SubAnnouncement Reconnection + Retention Sync

**目的:** 接続確立時の SubAnnouncement 自動交換、差分同期時の Retention 情報伝達。
**依存:** Phase 1 完了（Phase 2-3 と並行可能）

### Step 4.1: SubAnnouncement 再接続ハンドシェイク

| File | Operation | Description |
|------|-----------|-------------|
| `internal/groupshare/layer.go` | Modify | `AnnounceAllSubscriptions(ctx, peerDIDs)` 追加。LocalSubs の全エントリを送信 |
| `internal/node/node.go` | Modify | 認証完了コールバックで `AnnounceAllSubscriptions` を呼び出し |
| `internal/groupshare/groupshare_test.go` | Modify | 再接続時 SubAnnouncement テスト追加 |

### Step 4.2: 差分同期時の Retention 情報伝達

| File | Operation | Description |
|------|-----------|-------------|
| `internal/groupshare/types.go` | Modify | `RetentionInfo` 型追加（channel → duration map） |
| `internal/groupshare/layer.go` | Modify | `GetRetentionInfo()` 追加。差分同期ハンドシェイクに組み込み |
| `internal/devicesync/replication.go` | Modify | `SyncWith` で Retention 情報を考慮、期限切れレコードスキップ |

### Tests
```bash
go test ./internal/groupshare/... -v
go test ./internal/devicesync/... -v
go test ./test/integration/... -v -timeout 120s
```

---

## Phase 5: Permission Model (Role DAG + Table Permissions)

**目的:** テーブル単位の read/write/delete 権限。ロール DAG に基づくアクセス制御。
**依存:** Phase 2 完了

### Step 5.1: テーブル権限定義

| File | Operation | Description |
|------|-----------|-------------|
| `internal/permission/permission.go` | Create | `Permissions` 型、`Check(dag, memberRole, op)` で権限判定 |
| `internal/permission/permission_test.go` | Create | 権限判定テスト（ロール包含、self、owner、members） |

```go
type Permissions struct {
    Read   string // role name, "self", "owner", "members"
    Write  string
    Delete string
}
func Check(dag *role.DAG, memberRole, required string) bool
```

### Step 5.2: GroupShareLayer に権限統合

| File | Operation | Description |
|------|-----------|-------------|
| `internal/groupshare/layer.go` | Modify | Channel に Permissions 追加。Put/Get/Delete 時にロール DAG 権限チェック |
| `internal/groupshare/types.go` | Modify | `Channel` に `Perms *permission.Permissions` 追加 |

### Step 5.3: read 権限による同期範囲制御

| File | Operation | Description |
|------|-----------|-------------|
| `internal/groupshare/layer.go` | Modify | read=self → 自デバイスのみ、read=role → 該当ロール以上のメンバーのみに配信 |

### Tests
```bash
go test ./internal/permission/... -v
go test ./internal/groupshare/... -v
```

---

## Phase 6: SQL Query Interface

**目的:** アプリ向け最終 API。`client.DB().Exec()` / `Query()` を提供。
**依存:** Phase 2, 3, 5 完了

### Step 6.1: DB インターフェース定義

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | `DB` interface 追加 |

```go
type DB interface {
    Exec(ctx context.Context, sql string, args ...any) (Result, error)
    Query(ctx context.Context, sql string, args ...any) (*Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) *Row
    SetPermissions(ctx context.Context, table string, perms Permissions) error
    Migrate(ctx context.Context, migrations []Migration) error
}
type Migration struct { Version int; SQL string }
```

### Step 6.2: SQL プロキシ層

| File | Operation | Description |
|------|-----------|-------------|
| `internal/sqlproxy/proxy.go` | Create | SQL → ローカル SQLite 実行 → ChangeLog 記録 → 同期トリガー |
| `internal/sqlproxy/intercept.go` | Create | INSERT/UPDATE/DELETE を検知し同期対象に変換 |
| `internal/sqlproxy/proxy_test.go` | Create | 基本 CRUD テスト |

### Step 6.3: Client.DB() 統合

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/client.go` | Modify | `DB()` メソッド追加。Network.Select 後に利用可能 |

### Step 6.4: スキーマ同期 + 保留キュー

| File | Operation | Description |
|------|-----------|-------------|
| `internal/sqlproxy/schema.go` | Create | CREATE TABLE / ALTER TABLE をメタテーブルに記録・MyDB 経由で同期 |
| `internal/sqlproxy/pending.go` | Create | 保留キュー（上限あり、超過分は破棄→差分同期で再取得） |

### Step 6.5: daemon RPC 拡張

| File | Operation | Description |
|------|-----------|-------------|
| `cmd/linkself-daemon/main.go` | Modify | `db.exec`, `db.query`, `db.setPermissions`, `db.migrate` 追加 |

### Tests
```bash
go test ./internal/sqlproxy/... -v
go test ./pkg/linkself/ -timeout 120s
go test ./test/integration/... -v -timeout 120s
```

---

## Phase Dependencies

```
Phase 1 (API Rename + MyDB Dump)
    ↓
Phase 2 (Network + Role DAG)  ←→  Phase 4 (SubAnnounce + Retention Sync)
    ↓
Phase 3 (Storage Auto-Placement)
    ↓
Phase 5 (Permission Model)
    ↓
Phase 6 (SQL Query Interface)
```

Phase 4 は Phase 1 のみに依存し、Phase 2-3 と並行実行可能。

---

## Risks and Mitigation

| Risk | Mitigation |
|------|------------|
| Group→Network 移行で既存テスト破損 | 破壊的変更を許容。テストを一括更新。internal/group は internal/network に置換 |
| SQL プロキシ層の複雑性 | Phase 6 を最後に。それまで Put/Get API が内部で動作し続ける |
| SQLite WAL と複数プロセス同時アクセス | Phase 3 で WAL モード前提。複数プロセス問題は仕様どおり後続検討 |
| ロール DAG の循環参照 | NewDAG() 構築時にバリデーション。エラーを返す |
| 差分同期のプロトコル互換性 | 破壊的変更を許容。プロトコルバージョンを上げる |
| chat-client (Electron) の daemon RPC 呼び出し | RPC メソッド名変更に合わせて chat-client 側も一括更新 |

---

## Decision References

- [data-sync-concept.md §14](chat-client/docs/wants/data-sync-concept.md) — 全決定事項一覧
- [sync-db-plan.md §9](docs/sync-db-plan.md) — 用語の進化 + オーナー→ロール DAG
- [dump-restore-retention.md §8-10](docs/dump-restore-retention.md) — MyDB Dump, Retention Sync, 保留キュー
