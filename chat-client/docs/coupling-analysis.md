# DaemonとLinkSelf Coreライブラリの結合度分析

## 現在の結合度評価

### 結合度レベル: **中程度の結合**（部分的に疎結合、部分的に密結合）

## 結合度の詳細分析

### 1. 疎結合な部分 ✅

#### JSON-RPCインターフェース層
- **抽象化レベル**: 高い
- **通信プロトコル**: JSON-RPC 2.0（標準プロトコル）
- **プロセス間通信**: stdio経由で完全に分離
- **Electron側の依存**: JSON-RPCインターフェースのみ

```
Electron (TypeScript) 
    ↓ JSON-RPC (stdio)
Daemon (Go)
    ↓ Go API呼び出し
LinkSelf Core (internal)
```

**評価**: Electron側から見ると、daemonの実装詳細は完全に隠蔽されており、**非常に疎結合**。

### 2. 密結合な部分 ⚠️

#### DaemonとCoreの直接依存
```go
// core/cmd/linkself-daemon/main.go
import (
    "github.com/SeijiShii/link-self/core/internal/did"  // ⚠️ internalパッケージ
    "github.com/SeijiShii/link-self/core/internal/node" // ⚠️ internalパッケージ
)
```

**問題点**:
1. **`internal`パッケージへの直接依存**
   - Goの`internal`パッケージは同じモジュール内からのみアクセス可能
   - daemonは`core/cmd/linkself-daemon`に配置されているため、現在はアクセス可能
   - しかし、これは**実装詳細への依存**であり、API変更に弱い

2. **具体的な型への依存**
   - `*node.Node`型を直接使用
   - `*did.Identity`型を直接使用
   - `node.Config`構造体を直接使用

3. **将来の変更への脆弱性**
   - `internal`パッケージの構造変更に影響を受ける
   - Phase 2で公開APIが整備された場合、移行が必要

## 結合度の評価マトリックス

| 観点 | 評価 | 説明 |
|------|------|------|
| **Electron ↔ Daemon** | ✅ 疎結合 | JSON-RPC経由で完全に分離 |
| **Daemon ↔ Core** | ⚠️ 密結合 | `internal`パッケージへの直接依存 |
| **インターフェース抽象化** | ✅ 良好 | JSON-RPCで抽象化されている |
| **実装詳細の隠蔽** | ⚠️ 不十分 | `internal`パッケージの実装に依存 |
| **変更への耐性** | ⚠️ 中程度 | `internal`の変更に影響を受ける |

## 現在のアーキテクチャ

```
┌─────────────────────────────────────────┐
│  Electron (TypeScript)                  │
│  - React UI                             │
│  - IPC通信 (preload.ts)                 │
└──────────────┬──────────────────────────┘
               │ IPC (electron IPC)
               ↓
┌──────────────┴──────────────────────────┐
│  Electron Main Process (main.ts)        │
│  - JSON-RPC クライアント                │
└──────────────┬──────────────────────────┘
               │ stdio (JSON-RPC)
               ↓
┌──────────────┴──────────────────────────┐
│  Daemon (Go)                             │
│  - JSON-RPC サーバー                     │
│  - リクエストハンドラー                  │
└──────────────┬──────────────────────────┘
               │ Go API呼び出し
               ↓
┌──────────────┴──────────────────────────┐
│  LinkSelf Core (internal)               │
│  - internal/node                         │
│  - internal/did                           │
│  - internal/dht                          │
│  - internal/auth                         │
└──────────────────────────────────────────┘
```

## 改善提案

### 短期（現状維持 + ドキュメント化）

1. **現状のまま使用**
   - daemonは`core`モジュール内に配置されているため、`internal`パッケージへのアクセスは可能
   - JSON-RPCインターフェースは既に抽象化されている
   - Electron側から見ると疎結合

2. **ドキュメント化**
   - `internal`パッケージへの依存を明記
   - API変更時の影響範囲を文書化

### 中期（Phase 2での改善）

1. **公開APIパッケージの作成**
   ```go
   // core/pkg/linkself/client.go (提案)
   package linkself
   
   type Client struct {
       // 公開APIのみを提供
   }
   
   func NewClient(config Config) (*Client, error)
   func (c *Client) Start(ctx context.Context) error
   func (c *Client) SendMessage(ctx context.Context, peerDID string, payload []byte) error
   ```

2. **Daemonのリファクタリング**
   - `core/pkg/linkself`の公開APIのみを使用
   - `internal`パッケージへの直接依存を削除

### 長期（完全な疎結合）

1. **プラグインアーキテクチャ**
   - daemonを独立したモジュールとして分離
   - 公開APIのみに依存

2. **インターフェースベースの設計**
   - `core/pkg/linkself`でインターフェースを定義
   - 実装は`internal`に隠蔽

## 結論

### 現在の状態

- **Electron側から見た結合度**: ✅ **非常に疎結合**
  - JSON-RPCインターフェース経由で完全に分離
  - daemonの実装変更がElectron側に影響しない

- **Daemon側から見た結合度**: ⚠️ **密結合**
  - `internal`パッケージへの直接依存
  - Coreの実装変更に影響を受ける可能性

### 推奨事項

1. **現時点では許容範囲**
   - daemonは`core`モジュール内に配置されているため、`internal`へのアクセスは設計上問題ない
   - JSON-RPCインターフェースが適切に抽象化されている

2. **Phase 2での改善を推奨**
   - 公開APIパッケージ（`core/pkg/linkself`）の作成
   - daemonを公開APIのみに依存するようにリファクタリング

3. **他のアプリケーションへの影響**
   - 現在のdaemonは`core`モジュール内に配置されているため、他のアプリケーションからは使用できない
   - Phase 2で公開APIが整備されれば、他のアプリケーションも同じAPIを使用可能

## 参考

- [LinkSelfをライブラリとして使う](../../docs/using-linkself-as-library.md)
- [Phase 1設計](../../docs/phase1-design.md)
- [サンプルアプリ計画](../../docs/sample-chat-app-plan.md)
