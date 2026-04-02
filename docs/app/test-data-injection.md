# ダミーデータ投入ガイド

**概要:** アプリ開発時に、実際のP2P接続なしでUI・ロジックをテストするためのダミーデータ投入方法。

---

## セキュリティ上の原則

ダミーデータは **本物のデータと明確に区別** できなければならない。

| 区分 | 本物 | ダミー |
|------|------|--------|
| DID | `did:key:z6Mk...` | `did:test:<hex>` |
| メッセージ送信元 | P2P認証済みピア | `did:test:` のみ許可 |

- `InjectTestMessage` は `did:test:` プレフィックスのDIDのみ受け付ける
- `did:key:` を渡すとエラーになる（本物のピアとの混同を防止）
- `IsTestDID(did)` でダミーかどうかを判定できる

---

## 1. テストDIDの生成

### Go API

```go
did, err := linkself.GenerateTestDID()
// did = "did:test:a4965ec380ae9dd7..."

if linkself.IsTestDID(did) {
    // true — ダミーDID
}
```

### JSON-RPC (daemon)

```json
{"jsonrpc":"2.0", "method":"generateTestDID", "id":1}
```

レスポンス:
```json
{"jsonrpc":"2.0", "result":{"did":"did:test:a4965ec380ae9dd7..."}, "id":1}
```

- ノード起動前でも呼び出し可能
- 呼び出すたびに異なるDIDが生成される
- `did:key:` とは異なるフォーマットのため、P2P操作（`connect`, `sendMessage`）には使用不可

---

## 2. テストメッセージの注入

### Go API

```go
// ハンドラを登録
client.SetOnMessage(func(peerDID string, payload []byte) {
    fmt.Printf("from %s: %s\n", peerDID, payload)
})

// テストDIDからのメッセージを注入
testDID, _ := linkself.GenerateTestDID()
err := client.InjectTestMessage(ctx, testDID, []byte("hello"))
// → ハンドラが呼ばれる: from did:test:xxx: hello
```

### JSON-RPC (daemon)

```json
{
  "jsonrpc": "2.0",
  "method": "injectTestMessage",
  "params": {
    "fromDID": "did:test:0000000000000001",
    "payload": "hello from test"
  },
  "id": 2
}
```

- ノード起動後に呼び出し可能
- `fromDID` は `did:test:` で始まる必要がある（それ以外はエラー）
- 登録済みの `onMessage` ハンドラが直接呼ばれる
- ハンドラ未登録時はエラーにならない（何も起きない）

---

## 3. 既存APIを使ったダミーデータ投入

テストDID生成・メッセージ注入以外にも、既存のAPIでダミーデータを投入できる。

### KV データ（連絡先など）

```json
{
  "jsonrpc": "2.0",
  "method": "mydb.put",
  "params": {
    "table": "contacts",
    "recordID": "did:test:0000000000000001",
    "body": {"did":"did:test:0000000000000001", "name":"テスト太郎"}
  },
  "id": 3
}
```

### SQL データ

```json
{
  "jsonrpc": "2.0",
  "method": "mydb.exec",
  "params": {
    "sql": "INSERT INTO messages (id, from_did, body) VALUES (?, ?, ?)",
    "args": ["msg-1", "did:test:0000000000000001", "ダミーメッセージ"]
  },
  "id": 4
}
```

### 一括リストア（Dump/Restore）

既存データのスナップショットから復元:

```json
{
  "jsonrpc": "2.0",
  "method": "mydb.restore",
  "params": {
    "records": [
      {"id":"c1", "table":"contacts", "body":{"did":"did:test:aaa","name":"Alice"}, "timestamp":1700000000},
      {"id":"c2", "table":"contacts", "body":{"did":"did:test:bbb","name":"Bob"}, "timestamp":1700000000}
    ]
  },
  "id": 5
}
```

---

## 4. 開発時の典型的なワークフロー

```
1. ノードを起動（start）
2. generateTestDID でダミー連絡先のDIDを生成
3. mydb.put でダミー連絡先を登録
4. injectTestMessage でダミーメッセージを注入
5. UIが正しく表示されるか確認
```

### アプリ側でのガード例（TypeScript）

```typescript
function isTestDID(did: string): boolean {
  return did.startsWith("did:test:");
}

// UI表示時にダミーデータを明示
function formatContact(contact: Contact): string {
  const prefix = isTestDID(contact.did) ? "[TEST] " : "";
  return prefix + (contact.name || contact.did);
}
```
