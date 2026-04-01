# Implementation Plan: LinkSelf Spec-to-Code Alignment (v2)

## Overview

data-sync-decisions.md §4 の設計変更を実装する計画。TDD（テスト先行）で進める。
5 フェーズ構成、各フェーズ内の各ステップは Red→Green→Refactor サイクルでコミット可能。

**方針:** Red（テスト書く、失敗確認）→ Green（最小実装で通す）→ Refactor

---

## Current State Summary（2026-04 監査結果）

**実装済み（仕様と整合）:**
- Role DAG + Network Service + Permission（internal/role, network, permission）
- MyDB (KV: Put/Get/Delete/List/Dump/Restore)
- SharedDB (Channel + Topic + Subscription + Retention + Purge)
- DB (SQL: Exec/Query/QueryRow/Migrate via sqlproxy)
- SubAnnouncement 再接続ハンドシェイク
- SQLite3 ストレージバックエンド（WAL モード）
- Envelope ベースメッセージルーティング
- dataroot パッケージ
- ListTables() on DeviceStorage
- ネットワーク最低 1 人

**問題: 3系統の API が分離して動作**
- `client.DB()` — SQL だが同期されない（`:memory:` で独立）
- `client.MyDB()` — 同期されるが KV のみ
- `client.SharedDB()` — グループ共有だが独立 API

**未実装（新仕様で決定済み）:**
- API 統合（MyDB を唯一の公開 API に）
- SQL-同期接続（sqlproxy → DeviceSync/GroupShare）
- 同期スコープ（テーブル単位の ScopeDevice / ScopeNetwork）
- ChangeLog 保持ポリシー（MinSeq / TruncateChangeLog）
- SyncWith() ハンドシェイク + 全同期フォールバック
- ユーザー鍵 / デバイス鍵の2層構造 + ペアリング
- Config.SuiteID

---

## Phase A: ChangeLog 保持 + 全同期フォールバック

**目的:** ChangeLog の肥大化防止と、長期オフラインデバイスの自動復帰。
**依存:** なし（即着手可能）
**仕様参照:** data-sync-decisions.md §4 P5-P7, dump-restore-retention.md §11

### A-1: DeviceStorage に MinSeq / TruncateChangeLog 追加

**Red:**
```go
// internal/devicesync/mem_storage_test.go
func TestMemStorage_MinSeq_Empty(t *testing.T)
func TestMemStorage_MinSeq_AfterAppend(t *testing.T)
func TestMemStorage_TruncateChangeLog(t *testing.T)
func TestMemStorage_TruncateChangeLog_ChangesSinceReflects(t *testing.T)
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `internal/devicesync/types.go` | Modify | DeviceStorage に `MinSeq(ctx) (uint64, error)` と `TruncateChangeLog(ctx, minSeq uint64) error` 追加 |
| `internal/devicesync/mem_storage.go` | Modify | MemStorage 実装 |
| `internal/storage/sqlite/device_storage.go` | Modify | SQLite 版実装 |

### A-2: Config に ChangeLogRetention 追加

**Red:**
```go
// pkg/linkself/api_test.go or types_test.go
func TestChangeLogRetention_Defaults(t *testing.T)
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | `RetentionMode`, `ChangeLogRetention` 型、`Config.ChangeLogRetention` 追加 |

### A-3: ReplicationEngine に自動切り捨て組み込み

**Red:**
```go
// internal/devicesync/replication_test.go
func TestReplicationEngine_Put_TriggersRetention_TimeBased(t *testing.T)
func TestReplicationEngine_Put_TriggersRetention_CountBased(t *testing.T)
func TestReplicationEngine_DefaultRetention(t *testing.T)
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `internal/devicesync/replication.go` | Modify | RetentionPolicy を注入、Put/Delete 後に enforce |

### A-4: SyncWith ハンドシェイク + 全同期フォールバック

**Red:**
```go
// internal/devicesync/replication_test.go
func TestReplicationEngine_SyncWith_IncrementalSync(t *testing.T)
func TestReplicationEngine_SyncWith_GapDetected_FullSync(t *testing.T)
func TestReplicationEngine_SyncWith_NoPeer(t *testing.T)
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `internal/devicesync/replication.go` | Modify | SyncWith 実装: seq交換 → ギャップ検出 → 差分 or Dump/Restore |

### Tests
```bash
go test ./internal/devicesync/... -v
go test ./internal/storage/sqlite/... -v
go test ./pkg/linkself/ -timeout 120s
```

---

## Phase B: API 統合（MyDB を唯一の公開 API に）

**目的:** DB() と SharedDB() を廃止し、MyDB に SQL 能力を統合。
**依存:** なし（Phase A と並行可能）
**仕様参照:** data-sync-decisions.md §4 P1

### B-1: MyDB に SQL メソッド追加

**Red:**
```go
// pkg/linkself/api_test.go
func TestMyDB_Exec_CreateTable(t *testing.T)
func TestMyDB_Exec_Insert(t *testing.T)
func TestMyDB_Query_Select(t *testing.T)
func TestMyDB_QueryRow(t *testing.T)
func TestMyDB_Migrate(t *testing.T)
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | MyDB に Exec/Query/QueryRow/Migrate 追加 |
| `pkg/linkself/api.go` | Modify | myDB struct に proxy 統合、SQL メソッド実装 |

### B-2: DB() / SharedDB() を Client から削除

**Red:**
```go
func TestClient_MyDB_HasSQL(t *testing.T)
// コンパイル時に DB()/SharedDB() が存在しないことを保証
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | Client から DB()/SharedDB() 削除。DB/SharedDB interface 削除 |
| `pkg/linkself/client.go` | Modify | DB()/SharedDB() メソッド削除、sqlproxy を myDB 内部に |
| `pkg/linkself/api.go` | Modify | dbAPI/sharedDB struct 削除 |

### B-3: 既存テスト・daemon RPC の更新

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/api_test.go` | Modify | 旧 DB/SharedDB テストを MyDB テストに統合 |
| `pkg/linkself/client_test.go` | Modify | DB()/SharedDB() 参照削除 |
| `test/integration/*_test.go` | Modify | 統合テスト更新 |
| `cmd/linkself-daemon/main.go` | Modify | `db.*` → `mydb.exec/query/migrate`、`shareddb.*` 削除 |

### Tests
```bash
go test ./pkg/linkself/ -timeout 120s
go test ./cmd/linkself-daemon/ -v
go test ./test/integration/... -v -timeout 120s
```

---

## Phase C: SQL-同期接続 + 同期スコープ

**目的:** sqlproxy の書き込みを DeviceSync / GroupShare に接続。テーブル単位のスコープ設定。
**依存:** Phase B 完了
**仕様参照:** data-sync-decisions.md §4 P2, P4

### C-1: sqlproxy OnWrite フック

**Red:**
```go
// internal/sqlproxy/proxy_test.go
func TestProxy_OnWrite_Insert(t *testing.T)
func TestProxy_OnWrite_Update(t *testing.T)
func TestProxy_OnWrite_Delete(t *testing.T)
func TestProxy_OnWrite_SelectNoFire(t *testing.T)
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `internal/sqlproxy/proxy.go` | Modify | WriteEvent 型 + OnWrite コールバック追加 |

### C-2: SyncScope 型とメタデータ

**Red:**
```go
// pkg/linkself/api_test.go
func TestMyDB_SetSyncScope_Device(t *testing.T)
func TestMyDB_SetSyncScope_Network(t *testing.T)
func TestMyDB_SetSyncScope_IncludeExisting_True(t *testing.T)
func TestMyDB_SetSyncScope_IncludeExisting_False(t *testing.T)
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | SyncScope 型、SyncScopeOption、WithIncludeExisting 追加 |
| `pkg/linkself/api.go` | Modify | SetSyncScope 実装、スコープメタデータ管理 |

### C-3: 書き込みルーティング

**Red:**
```go
// test/integration/sync_scope_test.go
func TestSyncScope_Device_OnlySameUserDevices(t *testing.T)
func TestSyncScope_Network_BroadcastToMembers(t *testing.T)
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/api.go` | Modify | OnWrite でスコープ確認 → DeviceSync or GroupShare にルーティング |
| `pkg/linkself/client.go` | Modify | 受信ハンドラで sqlproxy SQLite にも反映 |

### Tests
```bash
go test ./internal/sqlproxy/... -v
go test ./pkg/linkself/ -timeout 120s
go test ./test/integration/... -v -timeout 120s
```

---

## Phase D: SuiteID + ストレージ自動配置

**目的:** Config.SuiteID の追加。ストレージパスの自動決定を SuiteID ベースに。
**依存:** Phase B 完了
**仕様参照:** data-sync-concept.md §3, §9, §10

### D-1: Config.SuiteID

**Red:**
```go
func TestClient_Start_WithSuiteID(t *testing.T)
func TestClient_Start_WithoutSuiteID(t *testing.T)
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | Config.SuiteID 追加 |
| `pkg/linkself/client.go` | Modify | Start() で SuiteID → dataroot.SuiteDir() でパス決定 |
| `internal/dataroot/dataroot.go` | Modify | SuiteDir に SuiteID 統合 |

### Tests
```bash
go test ./internal/dataroot/... -v
go test ./pkg/linkself/ -timeout 120s
```

---

## Phase E: ユーザー鍵 / デバイス鍵の2層構造

**目的:** 各デバイスに固有鍵。ユーザー鍵はペアリングで安全に転送。
**依存:** Phase A, B 完了（D と並行可能）
**仕様参照:** data-sync-concept.md §5.1

### E-1: Identity の2層化

**Red:**
```go
// internal/did/identity_test.go
func TestGenerateUserIdentity(t *testing.T)
func TestGenerateDeviceIdentity(t *testing.T)
func TestUserAndDeviceDID_AreDifferent(t *testing.T)
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `internal/did/identity.go` | Modify | UserIdentity / DeviceIdentity 分離 |

### E-2: ペアリングトークン

**Red:**
```go
// internal/pairing/token_test.go
func TestCreateToken_HasExpiry(t *testing.T)
func TestValidateToken_Valid(t *testing.T)
func TestValidateToken_Expired(t *testing.T)
func TestValidateToken_Tampered(t *testing.T)
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `internal/pairing/token.go` | Create | 時間制限トークン生成・検証 |

### E-3: ペアリングプロトコル

**Red:**
```go
// internal/pairing/protocol_test.go
func TestPairing_FullFlow(t *testing.T)
func TestPairing_ExpiredToken(t *testing.T)
func TestPairing_WrongToken(t *testing.T)
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `internal/pairing/protocol.go` | Create | ペアリングハンドシェイク実装 |

### E-4: 公開 API + 統合テスト

**Red:**
```go
// test/integration/pairing_test.go
func TestPairing_TwoDevices_SyncAfterPair(t *testing.T)
```

**Green:**

| File | Operation | Description |
|------|-----------|-------------|
| `pkg/linkself/types.go` | Modify | Client に CreatePairingToken/PairWithToken 追加 |
| `pkg/linkself/client.go` | Modify | ペアリング API 実装 |
| `internal/node/node.go` | Modify | 認証でユーザー鍵使用、デバイスリスト管理 |
| `internal/devicesync/replication.go` | Modify | PeerProvider を「ペアリング済みデバイス群」に変更 |

### Tests
```bash
go test ./internal/did/... -v
go test ./internal/pairing/... -v
go test ./pkg/linkself/ -timeout 120s
go test ./test/integration/... -v -timeout 120s
```

---

## Phase Dependencies

```
Phase A (ChangeLog保持 + 全同期)     Phase D (SuiteID + ストレージ)
    独立、即着手可能                      Phase B 完了後
                                          ↑
Phase B (API統合: MyDB唯一)  ──────→  Phase C (SQL-同期接続 + スコープ)
    独立、即着手可能

Phase E (ユーザー鍵/デバイス鍵)
    Phase A, B 完了後（D と並行可能）
```

**推奨着手順序:** A と B を並行 → C → D と E を並行

---

## Decision References

- [data-sync-concept.md](docs/spec/data-sync-concept.md) — 仕様本体
- [data-sync-decisions.md §4](docs/spec/data-sync-decisions.md) — 個人マルチデバイス精査による設計変更
- [sync-db-plan.md §1.3-1.4](docs/spec/sync-db-plan.md) — API統合方針、2層鍵方針
- [dump-restore-retention.md §11](docs/spec/dump-restore-retention.md) — ChangeLog保持ポリシー + 全同期フォールバック
