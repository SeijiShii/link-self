# Proposal: LinkSelf ストレージ実装の外部注入

**From:** home-visit-suite
**To:** LinkSelf
**Date:** 2026-03-31
**Status:** Draft

---

## 背景

home-visit-suite は LinkSelf をデータインフラとして利用する。
ストレージの実装責任はアプリ側にあり、home-visit-suite は SQLite3 で実装する予定。

現在の問題:

- ストレージインターフェース（`DeviceStorage`, `SharedStorage`, `SubscriptionStore`, `group.Store`, `RecordStorage`）が `core/internal/` に閉じており、外部モジュールからアクセスできない
- `client.go` の `Start()` が `NewMemStorage()` / `NewMemSharedStorage()` をハードコードしており、アプリ側の実装を注入できない

## 提案

### 1. ストレージインターフェースと関連型を `core/pkg/linkself` に公開する

以下のインターフェースと型を公開 API に追加する。
既存の internal 定義を re-export するか、pkg に移動する。

#### DeviceStorage（現在: `core/internal/devicesync`）

```go
// pkg/linkself/storage.go

type DeviceStorageRecord struct {
    ID        string
    Table     string
    Body      []byte
    Timestamp int64
}

type ChangeEntry struct {
    Seq       uint64
    Timestamp int64
    Table     string
    RecordID  string
    Op        int    // 0=Put, 1=Delete
    Body      []byte
}

type DeviceStorage interface {
    Put(ctx context.Context, table, id string, body []byte, timestamp int64) (uint64, error)
    Get(ctx context.Context, table, id string) (*DeviceStorageRecord, error)
    Delete(ctx context.Context, table, id string, timestamp int64) (uint64, error)
    List(ctx context.Context, table string) ([]*DeviceStorageRecord, error)
    GetTimestamp(ctx context.Context, table, id string) (int64, error)
    AppendChange(ctx context.Context, entry *ChangeEntry) error
    ChangesSince(ctx context.Context, since uint64) ([]*ChangeEntry, error)
    LatestSeq(ctx context.Context) (uint64, error)
}
```

#### SharedStorage（現在: `core/internal/groupshare`）

```go
type SharedStorageRecord struct {
    ID        string
    Channel   string
    Topic     string
    GroupID   string
    DID       string
    Timestamp int64
    Body      []byte
    Deleted   bool
}

type SharedStorage interface {
    PutShared(ctx context.Context, record *SharedStorageRecord) error
    GetShared(ctx context.Context, channel, id string) (*SharedStorageRecord, error)
    GetTimestamp(ctx context.Context, channel, id string) (int64, error)
    DeleteShared(ctx context.Context, channel, id string) error
    ListByChannel(ctx context.Context, channel string) ([]*SharedStorageRecord, error)
    ListByGroup(ctx context.Context, groupID string) ([]*SharedStorageRecord, error)
    ListByChannelAndTopic(ctx context.Context, channel, topic string) ([]*SharedStorageRecord, error)
    DeleteExpired(ctx context.Context, channel string, before int64) (int, error)
}
```

#### SubscriptionStore（現在: `core/internal/groupshare`）

```go
type SubscriptionStore interface {
    SetSubscription(did, channel string, topics []string) error
    GetSubscription(did, channel string) ([]string, error)
    GetAllSubscriptions(did string) (map[string][]string, error)
}
```

#### GroupStore（現在: `core/internal/group`）

```go
type GroupData struct {
    ID      string
    Members []string
    Owners  []string
}

type GroupStore interface {
    ListGroupIDsForMember(memberDID string) ([]string, error)
    GetGroup(groupID string) (*GroupData, error)
    CreateGroup(members []string, ownerDIDs []string) (groupID string, err error)
    UpdateGroup(groupID string, members []string, ownerDIDs []string) error
    DeleteGroup(groupID string) error
}
```

#### RecordStorage（現在: `core/internal/syncdb`）

```go
type SyncRecord struct {
    ID        string
    GroupID   string
    DID       string
    Timestamp int64
    Body      []byte
    Deleted   bool
}

type RecordStorage interface {
    Put(ctx context.Context, record *SyncRecord) error
    Get(ctx context.Context, id string) (*SyncRecord, error)
    GetTimestamp(ctx context.Context, id string) (int64, error)
    Delete(ctx context.Context, id string) error
    List(ctx context.Context) ([]*SyncRecord, error)
}
```

### 2. Config にストレージ注入フィールドを追加する

```go
type Config struct {
    IdentityPath   string
    ListenAddrs    []string
    BootstrapPeers []string

    // Storage injection — アプリが実装を提供する。
    // nil の場合は従来通り MemStorage を使用（後方互換）。
    DeviceStorage    DeviceStorage
    SharedStorage    SharedStorage
    SubscriptionStore SubscriptionStore
    GroupStore       GroupStore
    RecordStorage    RecordStorage
}
```

### 3. `client.go` の `Start()` を修正する

```go
// Start() 内の変更箇所（擬似コード）:

// DeviceSync
var dsStorage devicesync.DeviceStorage
if config.DeviceStorage != nil {
    dsStorage = adapt(config.DeviceStorage) // pkg型→internal型のアダプタ
} else {
    dsStorage = devicesync.NewMemStorage()
}

// GroupShare
var gsStorage groupshare.SharedStorage
if config.SharedStorage != nil {
    gsStorage = adapt(config.SharedStorage)
} else {
    gsStorage = groupshare.NewMemSharedStorage()
}

// 同様に GroupStore, SubscriptionStore, RecordStorage
```

internal型とpkg型の変換はLinkSelf内部のアダプタで吸収する。

## アプリ側の利用イメージ

```go
import (
    "github.com/SeijiShii/link-self/core/pkg/linkself"
)

// アプリがSQLite3で実装
sqliteDB := openSQLite("~/.home-visit-suite/linkself.db")
defer sqliteDB.Close()

client := linkself.NewClient()
info, err := client.Start(ctx, linkself.Config{
    IdentityPath:      "~/.home-visit-suite/identity.json",
    DeviceStorage:     sqliteDB.DeviceStorage(),
    SharedStorage:     sqliteDB.SharedStorage(),
    SubscriptionStore: sqliteDB.SubscriptionStore(),
    GroupStore:        sqliteDB.GroupStore(),
    RecordStorage:     sqliteDB.RecordStorage(),
})
```

## 後方互換性

- Config のストレージフィールドがすべて nil なら従来通り MemStorage を使用
- 既存のテスト・daemon は変更不要
- internal パッケージの型定義はそのまま残す（internal内での使用は継続）

## 参考: 既存の SQLite3 実装

LinkSelf には既に `core/internal/storage/sqlite/` に全5インターフェースの SQLite3 実装がある。
アプリが独自に実装する代わりに、この実装を公開ユーティリティとして提供する選択肢もある:

```go
// 案: pkg/linkself/sqlite パッケージとして公開
import "github.com/SeijiShii/link-self/core/pkg/linkself/sqlite"

db, _ := sqlite.Open("path/to/db")
client.Start(ctx, linkself.Config{
    DeviceStorage: db.DeviceStorage(),
    SharedStorage: db.SharedStorage(),
    // ...
})
```

この場合、アプリは独自実装の必要がなくなり、LinkSelfが品質保証したSQLite3実装をそのまま使える。
home-visit-suite としてはどちらでも対応可能。
