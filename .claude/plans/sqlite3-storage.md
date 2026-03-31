# Implementation Plan: SQLite3 Storage Layer

## Task Type
- [x] Backend (Go)

## Technical Solution

LinkSelfの5つのストレージインターフェース（RecordStorage, SharedStorage, DeviceStorage, SubscriptionStore, group.Store）に対して、SQLite3リファレンス実装を提供する。同時に、SharedStorageインターフェースにTopic絞り込み・Retention削除メソッドを追加し、データ量増加に備える。

### 設計方針
- **単一DBファイル**: 1ノード=1つのSQLite3ファイル（WALモード）
- **CGO-free**: `github.com/ncruces/go-sqlite3` (WASM-based, pure Go) を採用。クロスコンパイル容易
- **パッケージ構成**: `core/internal/storage/sqlite` に統合実装。各インターフェースを1つのDB上で実装
- **マイグレーション**: 組み込みスキーマバージョニング（`PRAGMA user_version`）
- **既存テスト流用**: インターフェーステスト（共通テストスイート）をMemStorageとSQLite3で両方実行

## Implementation Steps

### Step 1: SharedStorageインターフェース拡張
**目的**: Topic絞り込みとRetention削除をインターフェースに追加

1. `core/internal/groupshare/types.go` の `SharedStorage` に追加:
   ```go
   ListByChannelAndTopic(ctx context.Context, channel, topic string) ([]*SharedRecord, error)
   DeleteExpired(ctx context.Context, channel string, before int64) (int, error)
   ```
2. `MemSharedStorage` に対応メソッドを実装
3. `GroupShareLayer` に `Purge` の Retention 版メソッドを追加（`PurgeExpired`）
4. 既存テストが通ることを確認 + 新メソッドのテスト追加

**期待成果物**: 拡張されたインターフェースとMemStorage実装

### Step 2: RecordStorageインターフェース拡張
**目的**: 初期同期時の全件取得を可能にする

1. `core/internal/syncdb/storage.go` の `RecordStorage` に追加:
   ```go
   List(ctx context.Context) ([]*SyncRecord, error)
   ```
2. `MemStorage`（syncdb）に対応メソッドを実装
3. テスト追加

**期待成果物**: List付きRecordStorageとMemStorage実装

### Step 3: go-sqlite3依存追加とスキーマ定義
**目的**: SQLite3ドライバとスキーマを準備

1. `go get github.com/ncruces/go-sqlite3` を実行
2. `core/internal/storage/sqlite/schema.go` にスキーマ定義:
   ```sql
   -- Schema version 1
   PRAGMA journal_mode=WAL;
   PRAGMA foreign_keys=ON;

   CREATE TABLE IF NOT EXISTS sync_records (
       id        TEXT PRIMARY KEY,
       group_id  TEXT NOT NULL,
       did       TEXT NOT NULL,
       timestamp INTEGER NOT NULL,
       body      BLOB,
       deleted   INTEGER NOT NULL DEFAULT 0
   );
   CREATE INDEX IF NOT EXISTS idx_sync_records_group ON sync_records(group_id);

   CREATE TABLE IF NOT EXISTS shared_records (
       channel   TEXT NOT NULL,
       id        TEXT NOT NULL,
       topic     TEXT NOT NULL DEFAULT '',
       group_id  TEXT NOT NULL,
       did       TEXT NOT NULL,
       timestamp INTEGER NOT NULL,
       body      BLOB,
       deleted   INTEGER NOT NULL DEFAULT 0,
       PRIMARY KEY (channel, id)
   );
   CREATE INDEX IF NOT EXISTS idx_shared_channel_topic ON shared_records(channel, topic);
   CREATE INDEX IF NOT EXISTS idx_shared_group ON shared_records(group_id);
   CREATE INDEX IF NOT EXISTS idx_shared_timestamp ON shared_records(channel, timestamp);

   CREATE TABLE IF NOT EXISTS device_records (
       tbl       TEXT NOT NULL,
       id        TEXT NOT NULL,
       body      BLOB,
       timestamp INTEGER NOT NULL,
       PRIMARY KEY (tbl, id)
   );

   CREATE TABLE IF NOT EXISTS change_log (
       seq       INTEGER PRIMARY KEY AUTOINCREMENT,
       timestamp INTEGER NOT NULL,
       tbl       TEXT NOT NULL,
       record_id TEXT NOT NULL,
       op        INTEGER NOT NULL,
       body      BLOB
   );

   CREATE TABLE IF NOT EXISTS groups (
       id      TEXT PRIMARY KEY,
       members TEXT NOT NULL,  -- JSON array
       owners  TEXT NOT NULL   -- JSON array
   );

   CREATE TABLE IF NOT EXISTS subscriptions (
       did     TEXT NOT NULL,
       channel TEXT NOT NULL,
       topics  TEXT NOT NULL,  -- JSON array
       PRIMARY KEY (did, channel)
   );
   ```
3. `core/internal/storage/sqlite/migrate.go` にマイグレーションロジック:
   - `PRAGMA user_version` でバージョン管理
   - バージョン0→1: 初期スキーマ作成
   - 将来のバージョンアップに対応する構造

**期待成果物**: スキーマ定義とマイグレーション機能

### Step 4: SQLite3 RecordStorage実装
**目的**: syncdb.RecordStorage のSQLite3実装

1. `core/internal/storage/sqlite/record_storage.go`:
   - `Put`: INSERT OR REPLACE
   - `Get`: SELECT by id
   - `GetTimestamp`: SELECT timestamp by id
   - `Delete`: DELETE by id
   - `List`: SELECT all (Step 2で追加)
2. テスト: MemStorageと同じテストスイートを共有

**期待成果物**: RecordStorage SQLite3実装 + テスト

### Step 5: SQLite3 SharedStorage実装
**目的**: groupshare.SharedStorage のSQLite3実装

1. `core/internal/storage/sqlite/shared_storage.go`:
   - `PutShared`: INSERT OR REPLACE
   - `GetShared`: SELECT by (channel, id)
   - `GetTimestamp`: SELECT timestamp by (channel, id)
   - `DeleteShared`: DELETE by (channel, id)
   - `ListByChannel`: SELECT WHERE channel = ?
   - `ListByGroup`: SELECT WHERE group_id = ?
   - `ListByChannelAndTopic`: SELECT WHERE channel = ? AND topic = ?
   - `DeleteExpired`: DELETE WHERE channel = ? AND timestamp < ?; RETURN count
2. テスト: MemSharedStorageと同じテストスイートを共有

**期待成果物**: SharedStorage SQLite3実装 + テスト

### Step 6: SQLite3 DeviceStorage実装
**目的**: devicesync.DeviceStorage のSQLite3実装

1. `core/internal/storage/sqlite/device_storage.go`:
   - `Put`: INSERT OR REPLACE + INSERT INTO change_log
   - `Get`: SELECT by (tbl, id)
   - `Delete`: DELETE + INSERT INTO change_log
   - `List`: SELECT WHERE tbl = ?
   - `GetTimestamp`: SELECT timestamp by (tbl, id)
   - `AppendChange`: INSERT INTO change_log
   - `ChangesSince`: SELECT FROM change_log WHERE seq > ?
   - `LatestSeq`: SELECT MAX(seq) FROM change_log
2. **注意**: seq はAUTOINCREMENTで自動採番。Putの戻り値はlast_insert_rowid()
3. テスト

**期待成果物**: DeviceStorage SQLite3実装 + テスト

### Step 7: SQLite3 SubscriptionStore + group.Store実装
**目的**: 残り2つのインターフェースの実装

1. `core/internal/storage/sqlite/subscription_store.go`:
   - topics は JSON配列として保存
2. `core/internal/storage/sqlite/group_store.go`:
   - members/owners は JSON配列として保存
   - `ListGroupIDsForMember`: JSON_EACH で展開してクエリ
   - **代替案**: `group_memberships`正規化テーブル使用（パフォーマンス重視）
3. テスト

**期待成果物**: SubscriptionStore + group.Store SQLite3実装 + テスト

### Step 8: 統合DB管理と公開API
**目的**: 1つのDBコネクションで全ストレージを統合提供

1. `core/internal/storage/sqlite/db.go`:
   ```go
   type DB struct {
       conn *sql.DB
   }

   func Open(path string) (*DB, error)       // WALモード設定 + マイグレーション実行
   func (db *DB) Close() error
   func (db *DB) RecordStorage() syncdb.RecordStorage
   func (db *DB) SharedStorage() groupshare.SharedStorage
   func (db *DB) DeviceStorage() devicesync.DeviceStorage
   func (db *DB) SubscriptionStore() groupshare.SubscriptionStore
   func (db *DB) GroupStore() group.Store
   ```
2. `core/pkg/linkself` から必要に応じて公開（Config.DBPath等）

**期待成果物**: 統合DBファクトリ + 公開APIへの統合

### Step 9: 統合テストとドキュメント
**目的**: 全体結合の検証

1. `core/test/integration/sqlite_storage_test.go`:
   - 全インターフェースをSQLite3実装で結合テスト
   - Retention削除の動作確認
   - WALモードでの並行アクセステスト
2. 既存の統合テスト（syncdb, devicesync, groupshare）にSQLite3実装版を追加

**期待成果物**: 統合テスト + 動作確認

## Key Files

| File | Operation | Description |
|------|-----------|-------------|
| `core/internal/groupshare/types.go:46-54` | Modify | SharedStorageに2メソッド追加 |
| `core/internal/groupshare/mem_storage.go` | Modify | MemSharedStorageに新メソッド実装 |
| `core/internal/groupshare/layer.go` | Modify | PurgeExpiredメソッド追加 |
| `core/internal/syncdb/storage.go:9-14` | Modify | RecordStorageにList追加 |
| `core/internal/syncdb/memstorage.go` | Modify | MemStorageにList実装 |
| `core/internal/storage/sqlite/schema.go` | Create | スキーマ定義 |
| `core/internal/storage/sqlite/migrate.go` | Create | マイグレーション |
| `core/internal/storage/sqlite/db.go` | Create | DB統合管理 |
| `core/internal/storage/sqlite/record_storage.go` | Create | RecordStorage実装 |
| `core/internal/storage/sqlite/shared_storage.go` | Create | SharedStorage実装 |
| `core/internal/storage/sqlite/device_storage.go` | Create | DeviceStorage実装 |
| `core/internal/storage/sqlite/subscription_store.go` | Create | SubscriptionStore実装 |
| `core/internal/storage/sqlite/group_store.go` | Create | group.Store実装 |
| `core/internal/storage/sqlite/*_test.go` | Create | 各実装のテスト |
| `core/go.mod` | Modify | go-sqlite3依存追加 |

## Risks and Mitigation

| Risk | Mitigation |
|------|------------|
| `ncruces/go-sqlite3`のWASMオーバーヘッド | ベンチマークで測定。許容できない場合は`mattn/go-sqlite3`(CGO)にフォールバック |
| `group_memberships`のJSON_EACH性能 | 正規化テーブル（`group_memberships`）をフォールバックとして設計 |
| インターフェース変更による既存コード影響 | MemStorage実装も同時に更新。コンパイルエラーで検出可能 |
| DeviceStorageのseq番号: AUTOINCREMENT vs アプリ採番 | SQLite3実装はAUTOINCREMENT、AppendChangeでは明示的seq使用。両方サポート |
| WALモードの同時書き込み制限（単一writer） | P2Pノードは1プロセスなので問題なし |

## Architecture Decision Records

### ADR-1: Pure Go SQLite (ncruces) vs CGO (mattn)
- **選択**: `ncruces/go-sqlite3`（Pure Go / WASM）
- **理由**: クロスコンパイル（Android/iOS/Linux/Mac/Windows）が容易。CGO不要
- **トレードオフ**: WASM起動オーバーヘッドあり。性能問題時はmattn版に切替

### ADR-2: 単一DBファイル vs 分離DB
- **選択**: 1ノード = 1 SQLite3ファイル
- **理由**: トランザクション整合性、バックアップ容易性、接続管理の単純化
- **トレードオフ**: 大量データ時にロック競合の可能性（WALで緩和）

### ADR-3: group_membershipsの正規化
- **初期実装**: JSONカラム + JSON_EACH
- **理由**: スキーマが単純。メンバー数が少ない（通常2-10人）ので性能問題は生じにくい
- **エスカレーション**: 性能問題が出たら正規化テーブルに移行（マイグレーションv2）
