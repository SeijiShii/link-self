# Data Dump, Restore & Retention Management

[Japanese](dump-restore-retention.md) | **English** (this page)
**See also:** [Data Sync Design (DeviceSync / GroupShare)](sync-db-plan.en.md), [Topic Filtering](topic-subscription-filtering.en.md)

---

## 1. Overview

Adds **data dump/restore** and **retention period management** to the GroupShare layer.

### Motivation

- Need backup/migration capability for group-shared data
- Prevent storage bloat from unbounded accumulation of transactional data (e.g., chat messages)
- Master data (e.g., user profiles) must be stored permanently

### Design Principles

1. **Infra layer provides mechanisms**: Dump, restore, and purge facilities
2. **App layer decides permissions and retention**: Dump authorization is the app's responsibility. Retention period is set at channel registration
3. **LWW (Last-Write-Wins)**: Restore follows existing timestamp-based conflict resolution
4. **Independent P2P node management**: Each node manages its own storage cleanup independently

---

## 2. Dump & Restore

### 2.1 Design

| Item | Specification |
|------|---------------|
| Dump target | GroupShare only (DeviceDB excluded) |
| Dump scope | All channels for a given group ID |
| Dump format | `[]*SharedRecord` as JSON |
| Expired records | Excluded from dump |
| Restore conflict resolution | LWW (timestamp wins) |
| Permission check | Not performed at infra layer |

### 2.2 Permission Model

```
If the app layer has permission management:
  → Only group admin can dump (controlled via AccessPolicy, etc.)

If the app layer has no permission management:
  → All users can dump (default behavior)
```

### 2.3 API

#### Internal Layer (GroupShareLayer)

```go
// Dump returns all non-expired shared records for a group across all channels.
func (l *GroupShareLayer) Dump(ctx context.Context, groupID string) ([]*SharedRecord, error)

// Restore applies shared records using last-write-wins by timestamp.
// Returns the number of records actually applied (newer than existing).
func (l *GroupShareLayer) Restore(ctx context.Context, records []*SharedRecord) (int, error)
```

#### Public API (GroupShareAPI)

```go
Dump(ctx context.Context, groupID string) ([]*SharedRecord, error)
Restore(ctx context.Context, records []*SharedRecord) (int, error)
```

#### Daemon RPC

| Method | Parameters | Response |
|--------|------------|----------|
| `groupshare.dump` | `{"groupID": "..."}` | `[SharedRecord, ...]` |
| `groupshare.restore` | `{"records": [...]}` | `{"applied": N}` |

### 2.4 Restore LWW Flow

```
For each record in dump data:
  ├─ No existing record → Store
  ├─ Dump Timestamp > Existing Timestamp → Overwrite (count +1)
  └─ Dump Timestamp <= Existing Timestamp → Skip
```

---

## 3. Retention

### 3.1 Design

| Item | Specification |
|------|---------------|
| Configuration level | Per channel (Channel struct) |
| Expression | `time.Duration` (elapsed time from record Timestamp) |
| Permanent storage | `Retention = 0` (default = master data) |
| Expiry check | `now >= record.Timestamp + Retention.Milliseconds()` |
| Cleanup | Explicit `Purge` method (caller-driven) |
| Read-time filter | `Get`/`List`/`Dump` automatically exclude expired records |

### 3.2 Channel Struct

```go
type Channel struct {
    Name      string
    GroupID   string
    Schema    SchemaValidator // nil = accept any body
    Access    AccessPolicy    // nil = allow all
    Retention time.Duration   // 0 = permanent (master data)
}
```

### 3.3 Expiry Check

```go
func (l *GroupShareLayer) IsExpired(rec *SharedRecord, now int64) bool {
    ch, ok := l.channels[rec.Channel]
    if !ok || ch.Retention == 0 {
        return false
    }
    return now >= rec.Timestamp + ch.Retention.Milliseconds()
}
```

- Unknown channel → not considered expired (safe default)
- `Retention = 0` → permanent storage (master data)
- Each node can evaluate independently (Timestamp is shared across all nodes)

### 3.4 Impact on Methods

| Method | Handling of expired records |
|--------|-----------------------------|
| `Get` | Returns `nil, nil` (treated as non-existent) |
| `List` | Excluded from results |
| `Dump` | Excluded from results |
| `HandleIncoming` | Silently dropped on arrival (no error) |
| `Put` | No impact (new writes always accepted) |
| `Purge` | Physically deleted, count returned |

### 3.5 Purge

```go
func (l *GroupShareLayer) Purge(ctx context.Context, channel string) (int, error)
```

- Physically deletes expired records via `DeleteShared`
- `Retention = 0` channel → immediately returns `0, nil`
- Unregistered channel → `ErrChannelNotFound`
- Returns: number of records deleted

No background goroutines are introduced. The app layer calls Purge at appropriate times (startup, scheduled tasks, etc.).

---

## 4. Public API (pkg/linkself)

### 4.1 GroupShareAPI Interface

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

Passed as variadic arguments to `RegisterChannel`. Existing calls (no options) are unaffected.

### 4.3 Daemon RPC

| Method | Parameters | Response |
|--------|------------|----------|
| `groupshare.register` | `{"channel": "...", "groupID": "...", "retention": "720h"}` | none |
| `groupshare.dump` | `{"groupID": "..."}` | `[SharedRecord, ...]` |
| `groupshare.restore` | `{"records": [...]}` | `{"applied": N}` |
| `groupshare.purge` | `{"channel": "..."}` | `{"purged": N}` |

`retention` uses `time.ParseDuration` format (e.g., `"720h"` = 30 days, `"8760h"` = 365 days). Omitted = permanent.

---

## 5. Storage Interface

### 5.1 SharedStorage

```go
type SharedStorage interface {
    PutShared(ctx context.Context, record *SharedRecord) error
    GetShared(ctx context.Context, channel, id string) (*SharedRecord, error)
    GetTimestamp(ctx context.Context, channel, id string) (int64, error)
    DeleteShared(ctx context.Context, channel, id string) error
    ListByChannel(ctx context.Context, channel string) ([]*SharedRecord, error)
    ListByGroup(ctx context.Context, groupID string) ([]*SharedRecord, error)  // added
}
```

`ListByGroup` returns all records across all channels matching the group ID. Foundation for the dump operation.

---

## 6. Test Coverage

### Unit Tests

| Target | Tests | Coverage |
|--------|-------|----------|
| `MemSharedStorage.ListByGroup` | 1 | Group filtering, empty group |
| `GroupShareLayer.Dump` | 2 | Cross-channel dump, empty group, JSON round-trip |
| `GroupShareLayer.Restore` | 3 | New records, LWW (skip older / apply newer), empty input |
| `GroupShareLayer.IsExpired` | 3 | Permanent channel, with retention (within/boundary/past), unknown channel |
| `Get`/`List`/`Dump` filters | 4 | Expired excluded, non-expired returned, mixed list |
| `HandleIncoming` expiry rejection | 1 | Silently drops expired incoming records |
| `Purge` | 3 | Physical delete + count, permanent channel (0), unregistered channel (error) |

### Coverage: **89.1%**

---

## 7. Usage Examples

### 7.1 Chat Application

```go
// Master data: user profiles (permanent)
client.GroupShare().RegisterChannel("profiles", groupID)

// Transactional: chat messages (30-day retention)
client.GroupShare().RegisterChannel("messages", groupID,
    linkself.WithRetention(30 * 24 * time.Hour))

// Periodic cleanup (e.g., on app startup)
purged, _ := client.GroupShare().Purge(ctx, "messages")
fmt.Printf("%d expired messages cleaned up\n", purged)
```

### 7.2 Backup & Migration

```go
// Dump (assumes app layer has already checked admin permission)
records, _ := client.GroupShare().Dump(ctx, groupID)
jsonData, _ := json.Marshal(records)
os.WriteFile("backup.json", jsonData, 0644)

// Restore (on another node or for recovery)
var records []*linkself.SharedRecord
json.Unmarshal(jsonData, &records)
applied, _ := client.GroupShare().Restore(ctx, records)
fmt.Printf("%d records restored\n", applied)
```

### 7.3 Daemon RPC

```json
// Register channel (30-day retention)
{"jsonrpc":"2.0","method":"groupshare.register","params":{"channel":"messages","groupID":"g1","retention":"720h"},"id":1}

// Dump
{"jsonrpc":"2.0","method":"groupshare.dump","params":{"groupID":"g1"},"id":2}

// Restore
{"jsonrpc":"2.0","method":"groupshare.restore","params":{"records":[...]},"id":3}

// Cleanup
{"jsonrpc":"2.0","method":"groupshare.purge","params":{"channel":"messages"},"id":4}
```
