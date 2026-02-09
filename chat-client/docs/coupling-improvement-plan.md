# 密結合部分の改善計画

## 現状の問題

`core/cmd/linkself-daemon/main.go`が`internal`パッケージに直接依存している：

```go
import (
    "github.com/SeijiShii/link-self/core/internal/did"  // ⚠️ 密結合
    "github.com/SeijiShii/link-self/core/internal/node" // ⚠️ 密結合
)
```

## 改善アプローチ

### アプローチ1: 公開APIパッケージの作成（推奨）

`core/pkg/linkself`に公開APIパッケージを作成し、daemonは公開APIのみに依存するようにする。

#### 1. 公開APIパッケージの構造

```
core/
├── pkg/
│   └── linkself/
│       ├── client.go      # 公開APIクライアント
│       ├── types.go        # 公開型定義
│       └── identity.go    # Identity管理（公開API経由）
└── internal/
    └── ... (実装詳細)
```

#### 2. 公開APIインターフェース

```go
// core/pkg/linkself/types.go
package linkself

import "context"

// Config はLinkSelfノードの設定
type Config struct {
    IdentityPath   string
    ListenAddrs    []string
    BootstrapPeers []string
}

// NodeInfo はノードの情報
type NodeInfo struct {
    DID        string
    ListenAddr string
}

// MessageHandler はメッセージ受信時のコールバック
type MessageHandler func(peerDID string, payload []byte)

// Client はLinkSelfノードの公開API
type Client interface {
    Start(ctx context.Context, config Config) (*NodeInfo, error)
    Stop(ctx context.Context) error
    GetMyDID() string
    SendMessage(ctx context.Context, peerDID string, message string) error
    Connect(ctx context.Context, peerDID string) error
    SetOnMessage(handler MessageHandler)
}
```

#### 3. 実装（internalをラップ）

```go
// core/pkg/linkself/client.go
package linkself

import (
    "context"
    "fmt"
    
    "github.com/SeijiShii/link-self/core/internal/did"
    "github.com/SeijiShii/link-self/core/internal/node"
    "github.com/libp2p/go-libp2p/core/peer"
)

type client struct {
    node     *node.Node
    identity *did.Identity
}

func NewClient() Client {
    return &client{}
}

func (c *client) Start(ctx context.Context, config Config) (*NodeInfo, error) {
    // Identityの読み込み/生成
    identity, err := loadIdentity(config.IdentityPath)
    if err != nil {
        return nil, fmt.Errorf("load identity: %w", err)
    }
    
    // Bootstrap peersのパース
    var bootstrapPeers []peer.AddrInfo
    for _, addrStr := range config.BootstrapPeers {
        info, err := peer.AddrInfoFromString(addrStr)
        if err != nil {
            return nil, fmt.Errorf("parse bootstrap peer: %w", err)
        }
        bootstrapPeers = append(bootstrapPeers, *info)
    }
    
    // Nodeの作成
    cfg := node.Config{
        Identity:       identity,
        ListenAddrs:    config.ListenAddrs,
        BootstrapPeers: bootstrapPeers,
    }
    
    n, err := node.New(ctx, cfg)
    if err != nil {
        return nil, fmt.Errorf("create node: %w", err)
    }
    
    // Start
    if err := n.Start(ctx); err != nil {
        return nil, fmt.Errorf("start node: %w", err)
    }
    
    c.node = n
    c.identity = identity
    
    // NodeInfoの構築
    listenAddr := ""
    if len(n.Host.Addrs()) > 0 {
        listenAddr = fmt.Sprintf("%s/p2p/%s", n.Host.Addrs()[0].String(), n.Host.ID().String())
    }
    
    return &NodeInfo{
        DID:        identity.DID,
        ListenAddr: listenAddr,
    }, nil
}

func (c *client) Stop(ctx context.Context) error {
    if c.node != nil {
        return c.node.Close()
    }
    return nil
}

func (c *client) GetMyDID() string {
    if c.identity != nil {
        return c.identity.DID
    }
    return ""
}

func (c *client) SendMessage(ctx context.Context, peerDID string, message string) error {
    if c.node == nil {
        return fmt.Errorf("node not started")
    }
    memberDIDs := []string{c.identity.DID, peerDID}
    return c.node.SendToGroup(ctx, memberDIDs, []byte(message))
}

func (c *client) Connect(ctx context.Context, peerDID string) error {
    if c.node == nil {
        return fmt.Errorf("node not started")
    }
    _, err := c.node.Connect(ctx, peerDID)
    return err
}

func (c *client) SetOnMessage(handler MessageHandler) {
    if c.node != nil {
        c.node.SetOnMessage(handler)
    }
}
```

#### 4. Daemonのリファクタリング

```go
// core/cmd/linkself-daemon/main.go
package main

import (
    "github.com/SeijiShii/link-self/core/pkg/linkself" // ✅ 公開APIのみ
)

var (
    linkSelfClient linkself.Client // ✅ インターフェースに依存
)

func handleStart(req *JSONRPCRequest) {
    // ...
    
    client := linkself.NewClient()
    
    config := linkself.Config{
        IdentityPath:  identityPath,
        ListenAddrs:   listenAddrs,
        BootstrapPeers: params.BootstrapPeers,
    }
    
    info, err := client.Start(ctx, config)
    if err != nil {
        sendError(req.ID, -32000, "Failed to start", err.Error())
        return
    }
    
    linkSelfClient = client
    
    // Set message handler
    client.SetOnMessage(func(peerDID string, payload []byte) {
        sendNotification("onMessage", MessageNotificationParams{
            PeerDID: peerDID,
            Payload: string(payload),
        })
    })
    
    sendResponse(req.ID, StartResult{
        DID:        info.DID,
        ListenAddr: info.ListenAddr,
    })
}
```

### アプローチ2: インターフェースベースの設計

daemonが依存するインターフェースを定義し、実装は`internal`に隠蔽する。

#### メリット
- テスト容易性の向上（モック可能）
- 実装の交換が容易

#### デメリット
- インターフェースの定義とメンテナンスが必要
- やや複雑

### アプローチ3: 現状維持 + ドキュメント化

現状のまま使用し、依存関係を明確にドキュメント化する。

#### メリット
- 実装が簡単
- 変更不要

#### デメリット
- `internal`パッケージへの依存が続く
- Phase 2での移行が必要

## 推奨実装: アプローチ1（公開APIパッケージ）

### 実装ステップ

1. **`core/pkg/linkself`パッケージの作成**
   - `types.go`: 公開型定義
   - `client.go`: 公開API実装
   - `identity.go`: Identity管理（公開API経由）

2. **daemonのリファクタリング**
   - `internal`パッケージへの直接インポートを削除
   - `pkg/linkself`のみに依存

3. **テスト**
   - 公開APIのテスト
   - daemonの動作確認

### メリット

- ✅ **疎結合**: daemonは公開APIのみに依存
- ✅ **変更耐性**: `internal`の変更がdaemonに影響しない
- ✅ **再利用性**: 他のアプリケーションも同じAPIを使用可能
- ✅ **テスト容易性**: 公開APIをモック可能

### デメリット

- ⚠️ **実装コスト**: 公開APIパッケージの作成が必要
- ⚠️ **メンテナンス**: 公開APIの維持が必要

## 実装例

詳細な実装例は以下のファイルを参照：

- [core/pkg/linkself/types.go](../../core/pkg/linkself/types.go) - 公開API型定義
- [core/pkg/linkself/client.go](../../core/pkg/linkself/client.go) - 公開API実装
- [core/cmd/linkself-daemon/main_refactored.go.example](../../core/cmd/linkself-daemon/main_refactored.go.example) - daemonのリファクタリング例

### 主な変更点

#### Before (密結合)
```go
import (
    "github.com/SeijiShii/link-self/core/internal/did"  // ⚠️
    "github.com/SeijiShii/link-self/core/internal/node" // ⚠️
)

var linkSelfNode *node.Node // ⚠️ 実装詳細に依存

// 直接 internal パッケージを使用
identity, err := did.Generate()
cfg := node.Config{...}
n, err := node.New(ctx, cfg)
```

#### After (疎結合)
```go
import (
    "github.com/SeijiShii/link-self/core/pkg/linkself" // ✅ 公開APIのみ
)

var linkSelfClient linkself.Client // ✅ インターフェースに依存

// 公開APIを使用
client := linkself.NewClient()
config := linkself.Config{...}
info, err := client.Start(ctx, config)
```

## 実装手順

### ステップ1: 公開APIパッケージの作成

1. **`core/pkg/linkself/types.go`の作成**
   - `Client`インターフェースの定義
   - 公開型（`Config`, `NodeInfo`）の定義

2. **`core/pkg/linkself/client.go`の作成**
   - `Client`インターフェースの実装
   - `internal`パッケージへの依存をこのファイルに集約

### ステップ2: Daemonのリファクタリング

1. **インポートの変更**
   ```go
   // Before
   import "github.com/SeijiShii/link-self/core/internal/node"
   
   // After
   import "github.com/SeijiShii/link-self/core/pkg/linkself"
   ```

2. **変数の変更**
   ```go
   // Before
   var linkSelfNode *node.Node
   
   // After
   var linkSelfClient linkself.Client
   ```

3. **API呼び出しの変更**
   - `node.New()` → `linkself.NewClient().Start()`
   - `node.SendToGroup()` → `client.SendMessage()`
   - など

### ステップ3: テスト

1. 公開APIのユニットテスト
2. daemonの統合テスト
3. Electronアプリとの動作確認

## 比較表

| 観点 | Before (密結合) | After (疎結合) |
|------|----------------|----------------|
| **依存パッケージ** | `internal/did`, `internal/node` | `pkg/linkself` |
| **型の依存** | `*node.Node`, `*did.Identity` | `linkself.Client` (インターフェース) |
| **変更への耐性** | `internal`の変更に影響を受ける | 公開APIが安定していれば影響なし |
| **テスト容易性** | `internal`パッケージのモックが困難 | インターフェースなのでモック可能 |
| **再利用性** | `core`モジュール内からのみ使用可能 | 他のアプリケーションからも使用可能 |
| **コード行数** | daemon: ~376行 | daemon: ~250行（簡潔化） |

## 移行計画

### Phase 1（現状）
- 現状のまま使用
- ドキュメント化
- 公開APIパッケージの設計・実装

### Phase 2（改善）
1. `core/pkg/linkself`パッケージの作成 ✅（実装済み）
2. daemonのリファクタリング（`main_refactored.go.example`を参考）
3. テストと検証

### Phase 3（完全移行）
- 他のアプリケーションも公開APIを使用
- `internal`パッケージへの直接依存を完全に排除
- 公開APIのバージョニングとドキュメント化
