# Topic-Based Subscription Filtering

[日本語](topic-subscription-filtering.md) | **English** (this page)
**See also:** [Data Sync Design (DeviceSync / GroupShare)](sync-db-plan.en.md), [Group Concept](network-concept.en.md)

---

## 1. Overview

Introduces **topic-based subscription filtering** to the GroupShare layer.
Each device declares the scope of data it wants to receive at the topic level, and the sender selectively transmits data based on those declarations.

### Background & Motivation

- In serverless P2P infrastructure, sending all data to all devices wastes bandwidth and storage
- Need to optimize data transfer at the infra layer while maintaining the design principle of delegating permission management to the app layer
- Example: Editorial staff access all 100 documents; general staff receive only assigned documents

### Design Principles

1. **Sender-side filtering**: Exclude unnecessary data before transmission to save bandwidth
2. **Safe defaults**: No subscription registered → send (prevents data loss)
3. **Receiver-side CanRead as safety net**: Existing AccessPolicy checks remain in effect
4. **Compatible with app-layer permission delegation**: Infra provides filtering mechanism only; the app decides what to filter

---

## 2. Data Model

### 2.1 Topic Field in SharedRecord

```go
type SharedRecord struct {
    ID        string `json:"id"`
    Channel   string `json:"channel"`
    Topic     string `json:"topic,omitempty"` // topic for subscription filtering
    GroupID   string `json:"group_id"`
    DID       string `json:"did"`
    Timestamp int64  `json:"timestamp"`
    Body      []byte `json:"body"`
    Deleted   bool   `json:"deleted"`
}
```

- `Topic` is optional (empty string = no topic)
- App specifies any string (e.g., document ID `"doc-001"`)
- Used for sub-filtering within a channel

### 2.2 SubAnnouncement

```go
type SubAnnouncement struct {
    DID     string   `json:"did"`
    Channel string   `json:"channel"`
    Topics  []string `json:"topics"`
}
```

Message for exchanging subscription state between peers.

---

## 3. Subscription Store

### 3.1 Interface

```go
type SubscriptionStore interface {
    SetSubscription(did, channel string, topics []string) error
    GetSubscription(did, channel string) ([]string, error)
    GetAllSubscriptions(did string) (map[string][]string, error)
}
```

### 3.2 Implementations

| Implementation | Purpose | Persistence |
|---------------|---------|-------------|
| `MemSubscriptionStore` | Remote peers' subscriptions (RemoteSubs) | In-memory (volatile) |
| `DeviceSyncSubscriptionStore` | Own subscriptions (LocalSubs) | Auto-replicated across same-DID devices via DeviceSync |

### 3.3 DeviceSyncSubscriptionStore Details

- Uses DeviceSync's `ReplicationEngine` for persistence
- Table name: `_groupshare_subs`
- Record ID format: `{did}::{channel}` (e.g., `did:key:zAlice::docs`)
- Body: JSON `{"topics": ["doc-001", "doc-002"]}`
- Automatically replicated to other devices with the same DID

---

## 4. Filtering Logic

### 4.1 Topic Matching

```go
func TopicMatches(subscribed []string, topic string) bool
```

| subscribed | topic | result | reason |
|------------|-------|--------|--------|
| `["*"]` | any | `true` | wildcard |
| `["doc-001", "doc-002"]` | `"doc-001"` | `true` | exact match |
| `["doc-001"]` | `"doc-002"` | `false` | no match |
| `[]` | any | `false` | empty list = reject all |
| `["*"]` | `""` | `true` | wildcard matches empty topic |

### 4.2 broadcast() Filtering Flow

```
Get target member list
  ↓
RemoteSubs is nil → send to all (filtering disabled)
  ↓
For each member:
  ├─ Subscription not registered (nil) → send (safe default)
  ├─ TopicMatches = true → send
  └─ TopicMatches = false → skip
  ↓
Send to filtered members
```

**Safe default**: When subscription is not registered, data is sent. This ensures:
- Data is delivered even for apps that don't use subscription filtering
- No data loss when SubAnnouncement hasn't arrived due to network delay
- Receiver-side `AccessPolicy.CanRead()` acts as the ultimate safety net

---

## 5. SubAnnouncement Protocol

### 5.1 Envelope Type

```go
const TypeSubAnnounce Type = "sub_announce"
```

Uses a separate envelope type from GroupShare data messages (`TypeGroupShare`).

### 5.2 Flow

```
1. App calls Subscribe(channel, topics)
2. Save to LocalSubs (replicated to same-DID devices via DeviceSync)
3. Get group members via MemberResolver
4. JSON-encode SubAnnouncement
5. Wrap in TypeSubAnnounce envelope
6. Broadcast to all members
```

### 5.3 Receiver-Side Processing

```
1. Receive TypeSubAnnounce envelope
2. Call HandleSubAnnouncement(senderDID, payload)
3. JSON-decode to SubAnnouncement
4. DID validation: senderDID == announcement.DID (reject mismatch = spoofing prevention)
5. Save to RemoteSubs
```

---

## 6. Public API (pkg/linkself)

### 6.1 GroupShareAPI Interface

```go
type GroupShareAPI interface {
    RegisterChannel(name, groupID string) error
    Subscribe(channel string, topics []string) error
    Put(ctx context.Context, channel, topic, recordID string, body []byte) error
    Get(ctx context.Context, channel, recordID string) (*SharedRecord, error)
    Delete(ctx context.Context, channel, topic, recordID string) error
    List(ctx context.Context, channel string) ([]*SharedRecord, error)
}
```

- Added `topic` parameter to `Put` / `Delete`
- Added new `Subscribe` method

### 6.2 Daemon RPC

| Method | Parameters |
|--------|------------|
| `groupshare.put` | `channel`, `topic`, `record_id`, `body` |
| `groupshare.delete` | `channel`, `topic`, `record_id` |
| `groupshare.subscribe` | `channel`, `topics` |

---

## 7. Wiring (client.go)

```
GroupShareLayer
├── LocalSubs  = DeviceSyncSubscriptionStore(dsEngine)  ← persisted via DeviceSync
├── RemoteSubs = MemSubscriptionStore()                  ← in-memory
└── SendSubAnnounce = envelope.Wrap(TypeSubAnnounce) → node.SendToGroup

MessageRouter
├── OnGroupShare  → gsLayer.HandleIncoming
└── OnSubAnnounce → gsLayer.HandleSubAnnouncement
```

---

## 8. Test Coverage

### Unit Tests

| Target | Count | Coverage |
|--------|-------|----------|
| `TopicMatches` | 8 | wildcard, exact match, no match, empty list, empty topic |
| `MemSubscriptionStore` | 4 | Set/Get, overwrite, unsubscribe, GetAll |
| `DeviceSyncSubscriptionStore` | 6 | Set/Get, overwrite, unsubscribe, GetAll, multi-DID, replication sync |
| `GroupShareLayer` filtering | 4 | subscription filter, RemoteSubs nil, empty subscription excluded, Delete filter |
| `Subscribe` | 3 | happy path, LocalSubs nil, LocalSubs persistence |
| `HandleSubAnnouncement` | 3 | happy path, DID mismatch rejection, RemoteSubs nil |

### Integration Tests

- `groupshare_integration_test.go`: Existing tests updated to pass empty topic to Put calls, confirming backward compatibility

### Coverage: **89.2%**

---

## 9. Usage Example: Document Management App

```
Channel: "documents"
Topic: document ID (e.g., "doc-001", "doc-002", ...)

[Editorial Staff]
  Subscribe("documents", ["*"])  → receive all documents

[General Staff A (assigned doc-001, doc-003)]
  Subscribe("documents", ["doc-001", "doc-003"])  → receive only assigned docs

[General Staff B (assigned doc-002)]
  Subscribe("documents", ["doc-002"])  → receive only doc-002
```

The sender references each member's subscription and sends only necessary data.
For members without a registered subscription, all data is sent as a safe default.
