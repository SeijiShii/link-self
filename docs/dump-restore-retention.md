# データダンプ・リストア・保存期間管理

**日本語**（このページ）| [English](dump-restore-retention.en.md)
**参照:** [データ同期設計（DeviceSync / GroupShare）](sync-db-plan.md)、[トピックフィルタリング](topic-subscription-filtering.md)

---

## 1. 概要

GroupShare レイヤーに**データダンプ・リストア**と**保存期間（Retention）管理**の仕組みを追加する。

### 背景・動機

- グループ共有データのバックアップ・移行手段が必要
- トランザクショナルデータ（チャットメッセージ等）の無制限な蓄積によるストレージ肥大化を防ぎたい
- マスタデータ（ユーザープロフィール等）は永久保存が必要

### 設計原則

1. **インフラ層はメカニズムを提供**: ダンプ・リストア・Purge の仕組みを提供
2. **アプリ層が権限と保存期間を決定**: ダンプ権限のチェックはアプリ層の責務。保存期間はチャネル登録時にアプリが指定
3. **LWW（Last-Write-Wins）**: リストアは既存のタイムスタンプ競合解決ルールに従う
4. **P2P ノード独立管理**: 各ノードが自身のストレージを独立にクリーンアップ

---

## 2. ダンプ・リストア

### 2.1 設計

| 項目 | 仕様 |
|------|------|
| ダンプ対象 | GroupShare のみ（DeviceDB は対象外） |
| ダンプ単位 | グループ ID 指定で全チャネル横断 |
| ダンプ形式 | `[]*SharedRecord` の JSON |
| 期限切れレコード | ダンプから除外 |
| リストア競合解決 | LWW（タイムスタンプ後勝ち） |
| 権限チェック | インフラ層では行わない |

### 2.2 権限モデル

```
アプリ層に権限管理がある場合:
  → グループの管理者のみダンプ可能（AccessPolicy 等で制御）

アプリ層に権限管理がない場合:
  → 全てのユーザーがダンプ可能（デフォルト動作）
```

### 2.3 API

#### internal 層 (GroupShareLayer)

```go
// Dump returns all non-expired shared records for a group across all channels.
func (l *GroupShareLayer) Dump(ctx context.Context, groupID string) ([]*SharedRecord, error)

// Restore applies shared records using last-write-wins by timestamp.
// Returns the number of records actually applied (newer than existing).
func (l *GroupShareLayer) Restore(ctx context.Context, records []*SharedRecord) (int, error)
```

#### 公開 API (GroupShareAPI)

```go
Dump(ctx context.Context, groupID string) ([]*SharedRecord, error)
Restore(ctx context.Context, records []*SharedRecord) (int, error)
```

#### デーモン RPC

| メソッド | パラメータ | 戻り値 |
|----------|------------|--------|
| `groupshare.dump` | `{"groupID": "..."}` | `[SharedRecord, ...]` |
| `groupshare.restore` | `{"records": [...]}` | `{"applied": N}` |

### 2.4 リストアの LWW フロー

```
ダンプデータの各レコードについて:
  ├─ 既存レコードなし → 保存
  ├─ ダンプの Timestamp > 既存の Timestamp → 上書き（カウント +1）
  └─ ダンプの Timestamp <= 既存の Timestamp → スキップ
```

---

## 3. 保存期間（Retention）

### 3.1 設計

| 項目 | 仕様 |
|------|------|
| 設定単位 | チャネルレベル（Channel 構造体） |
| 表現方法 | `time.Duration`（レコードの Timestamp からの経過時間） |
| 永久保存 | `Retention = 0`（デフォルト = マスタデータ） |
| 期限切れ判定 | `now >= record.Timestamp + Retention.Milliseconds()` |
| クリーンアップ | 明示的 `Purge` メソッド（caller-driven） |
| 読み取りフィルタ | `Get`/`List`/`Dump` で期限切れを自動除外 |

### 3.2 Channel 構造体

```go
type Channel struct {
    Name      string
    GroupID   string
    Schema    SchemaValidator // nil = accept any body
    Access    AccessPolicy    // nil = allow all
    Retention time.Duration   // 0 = permanent (master data)
}
```

### 3.3 期限切れ判定

```go
func (l *GroupShareLayer) IsExpired(rec *SharedRecord, now int64) bool {
    ch, ok := l.channels[rec.Channel]
    if !ok || ch.Retention == 0 {
        return false
    }
    return now >= rec.Timestamp + ch.Retention.Milliseconds()
}
```

- チャネル未登録 → 期限切れとみなさない（安全なデフォルト）
- `Retention = 0` → 永久保存（マスタデータ）
- 判定は各ノードが独立に実行可能（Timestamp は全ノード共通）

### 3.4 影響範囲

| メソッド | 期限切れレコードの扱い |
|----------|------------------------|
| `Get` | `nil, nil` を返す（存在しないものとして扱う） |
| `List` | 結果から除外 |
| `Dump` | 結果から除外 |
| `HandleIncoming` | 到着時に期限切れなら静かに破棄（エラーなし） |
| `Put` | 影響なし（新規書き込みは常に受け入れ） |
| `Purge` | 物理削除して件数を返す |

### 3.5 Purge

```go
func (l *GroupShareLayer) Purge(ctx context.Context, channel string) (int, error)
```

- 指定チャネルの期限切れレコードを `DeleteShared` で物理削除
- `Retention = 0` のチャネル → 即座に `0, nil` を返す
- 未登録チャネル → `ErrChannelNotFound`
- 戻り値: 削除された件数

バックグラウンド goroutine は導入しない。アプリ層が適切なタイミング（起動時、定期タスク等）で呼び出す。

---

## 4. 公開 API（pkg/linkself）

### 4.1 GroupShareAPI インターフェース

```go
type GroupShareAPI interface {
    RegisterChannel(name, groupID string, opts ...ChannelOption) error
    Subscribe(channel string, topics []string) error
    Put(ctx context.Context, channel, topic, recordID string, body []byte) error
    Get(ctx context.Context, channel, recordID string) (*SharedRecord, error)
    Delete(ctx context.Context, channel, topic, recordID string) error
    List(ctx context.Context, channel string) ([]*SharedRecord, error)
    Dump(ctx context.Context, groupID string) ([]*SharedRecord, error)
    Restore(ctx context.Context, records []*SharedRecord) (int, error)
    Purge(ctx context.Context, channel string) (int, error)
}
```

### 4.2 ChannelOption

```go
type ChannelOption func(*channelConfig)

func WithRetention(d time.Duration) ChannelOption
```

`RegisterChannel` に可変長引数で渡す。既存の呼び出し（オプションなし）は影響を受けない。

### 4.3 デーモン RPC

| メソッド | パラメータ | 戻り値 |
|----------|------------|--------|
| `groupshare.register` | `{"channel": "...", "groupID": "...", "retention": "720h"}` | なし |
| `groupshare.dump` | `{"groupID": "..."}` | `[SharedRecord, ...]` |
| `groupshare.restore` | `{"records": [...]}` | `{"applied": N}` |
| `groupshare.purge` | `{"channel": "..."}` | `{"purged": N}` |

`retention` は `time.ParseDuration` 形式（例: `"720h"` = 30日、`"8760h"` = 365日）。省略時は永久保存。

---

## 5. ストレージインターフェース

### 5.1 SharedStorage

```go
type SharedStorage interface {
    PutShared(ctx context.Context, record *SharedRecord) error
    GetShared(ctx context.Context, channel, id string) (*SharedRecord, error)
    GetTimestamp(ctx context.Context, channel, id string) (int64, error)
    DeleteShared(ctx context.Context, channel, id string) error
    ListByChannel(ctx context.Context, channel string) ([]*SharedRecord, error)
    ListByGroup(ctx context.Context, groupID string) ([]*SharedRecord, error)  // 追加
}
```

`ListByGroup` はグループ ID に一致する全チャネルのレコードを返す。ダンプの基盤メソッド。

---

## 6. テストカバレッジ

### ユニットテスト

| テスト対象 | テスト数 | カバー範囲 |
|------------|----------|------------|
| `MemSharedStorage.ListByGroup` | 1 | グループ別リスト、空グループ |
| `GroupShareLayer.Dump` | 2 | グループ横断ダンプ、空グループ、JSON ラウンドトリップ |
| `GroupShareLayer.Restore` | 3 | 新規レコード、LWW（古いのスキップ/新しいの適用）、空入力 |
| `GroupShareLayer.IsExpired` | 3 | 永久チャネル、Retention あり（期限内/境界/期限後）、未登録チャネル |
| `Get`/`List`/`Dump` フィルタ | 4 | 期限切れ除外、非期限切れ返却、混在リスト |
| `HandleIncoming` 期限切れ拒否 | 1 | 到着時に期限切れのレコードを静かに破棄 |
| `Purge` | 3 | 物理削除＋件数、永久チャネル（0件）、未登録チャネル（エラー） |

### カバレッジ: **89.1%**

---

## 7. 利用例

### 7.1 チャットアプリ

```go
// マスタデータ: ユーザープロフィール（永久保存）
client.GroupShare().RegisterChannel("profiles", groupID)

// トランザクショナル: チャットメッセージ（30日保存）
client.GroupShare().RegisterChannel("messages", groupID,
    linkself.WithRetention(30 * 24 * time.Hour))

// 定期的なクリーンアップ（アプリ起動時など）
purged, _ := client.GroupShare().Purge(ctx, "messages")
fmt.Printf("%d expired messages cleaned up\n", purged)
```

### 7.2 バックアップ・移行

```go
// ダンプ（管理者がアプリ層で権限チェック済みの前提）
records, _ := client.GroupShare().Dump(ctx, groupID)
jsonData, _ := json.Marshal(records)
os.WriteFile("backup.json", jsonData, 0644)

// リストア（別ノードまたは復元時）
var records []*linkself.SharedRecord
json.Unmarshal(jsonData, &records)
applied, _ := client.GroupShare().Restore(ctx, records)
fmt.Printf("%d records restored\n", applied)
```

### 7.3 デーモン RPC

```json
// チャネル登録（30日保存）
{"jsonrpc":"2.0","method":"groupshare.register","params":{"channel":"messages","groupID":"g1","retention":"720h"},"id":1}

// ダンプ
{"jsonrpc":"2.0","method":"groupshare.dump","params":{"groupID":"g1"},"id":2}

// リストア
{"jsonrpc":"2.0","method":"groupshare.restore","params":{"records":[...]},"id":3}

// クリーンアップ
{"jsonrpc":"2.0","method":"groupshare.purge","params":{"channel":"messages"},"id":4}
```
