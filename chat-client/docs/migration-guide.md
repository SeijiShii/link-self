# リファクタリング移行ガイド

このガイドでは、`internal`パッケージへの直接依存から公開API (`core/pkg/linkself`) への移行方法を説明します。

## 概要

リファクタリングにより、daemonは`internal`パッケージへの直接依存を削除し、公開API (`core/pkg/linkself`) のみに依存するようになりました。この変更により、以下のメリットが得られます：

- **疎結合**: `internal`の実装詳細に依存しない
- **変更耐性**: `internal`の変更がアプリケーションに影響しない
- **再利用性**: 他のアプリケーションも同じAPIを使用可能
- **テスト容易性**: インターフェースなのでモック可能

## 変更内容

### Before (リファクタリング前)

```go
package main

import (
    "github.com/SeijiShii/link-self/core/internal/did"
    "github.com/SeijiShii/link-self/core/internal/node"
    "github.com/libp2p/go-libp2p/core/crypto"
    "github.com/libp2p/go-libp2p/core/peer"
)

var linkSelfNode *node.Node

func handleStart(req *JSONRPCRequest) {
    // 直接 internal パッケージを使用
    identity, err := loadOrGenerateIdentity(identityPath)
    // ...
    
    cfg := node.Config{
        Identity:       identity,
        ListenAddrs:    listenAddrs,
        BootstrapPeers: bootstrapPeers,
    }
    
    n, err := node.New(ctx, cfg)
    // ...
    linkSelfNode = n
}
```

### After (リファクタリング後)

```go
package main

import (
    "github.com/SeijiShii/link-self/core/pkg/linkself"
)

var linkSelfClient linkself.Client

func handleStart(req *JSONRPCRequest) {
    // 公開APIを使用
    client := linkself.NewClient()
    
    config := linkself.Config{
        IdentityPath:  params.IdentityPath,
        ListenAddrs:   params.ListenAddrs,
        BootstrapPeers: params.BootstrapPeers,
    }
    
    info, err := client.Start(ctx, config)
    // ...
    linkSelfClient = client
}
```

## 移行手順

### 1. インポートの変更

**削除:**
```go
import (
    "github.com/SeijiShii/link-self/core/internal/did"
    "github.com/SeijiShii/link-self/core/internal/node"
    "github.com/libp2p/go-libp2p/core/crypto"
    "github.com/libp2p/go-libp2p/core/peer"
)
```

**追加:**
```go
import (
    "github.com/SeijiShii/link-self/core/pkg/linkself"
)
```

### 2. 変数の変更

**Before:**
```go
var linkSelfNode *node.Node
```

**After:**
```go
var linkSelfClient linkself.Client
```

### 3. ノードの起動

**Before:**
```go
identity, err := loadOrGenerateIdentity(identityPath)
if err != nil {
    return err
}

var bootstrapPeers []peer.AddrInfo
for _, addrStr := range params.BootstrapPeers {
    info, err := peer.AddrInfoFromString(addrStr)
    // ...
    bootstrapPeers = append(bootstrapPeers, *info)
}

cfg := node.Config{
    Identity:       identity,
    ListenAddrs:    listenAddrs,
    BootstrapPeers: bootstrapPeers,
}

n, err := node.New(ctx, cfg)
if err != nil {
    return err
}

n.SetOnMessage(func(peerDID string, payload []byte) {
    // ...
})

if err := n.Start(ctx); err != nil {
    return err
}

linkSelfNode = n
```

**After:**
```go
client := linkself.NewClient()

config := linkself.Config{
    IdentityPath:  params.IdentityPath,
    ListenAddrs:   params.ListenAddrs,
    BootstrapPeers: params.BootstrapPeers,
}

info, err := client.Start(ctx, config)
if err != nil {
    return err
}

client.SetOnMessage(func(peerDID string, payload []byte) {
    // ...
})

linkSelfClient = client
```

### 4. ノードの停止

**Before:**
```go
if linkSelfNode != nil {
    linkSelfNode.Close()
    linkSelfNode = nil
}
```

**After:**
```go
if linkSelfClient != nil {
    linkSelfClient.Stop(ctx)
    linkSelfClient = nil
}
```

### 5. DIDの取得

**Before:**
```go
if linkSelfNode == nil {
    return "", errors.New("node not started")
}
return linkSelfNode.Identity.DID
```

**After:**
```go
if linkSelfClient == nil {
    return "", errors.New("node not started")
}
return linkSelfClient.GetMyDID()
```

### 6. メッセージの送信

**Before:**
```go
memberDIDs := []string{linkSelfNode.Identity.DID, params.PeerDID}
err := linkSelfNode.SendToGroup(ctx, memberDIDs, []byte(params.Message))
```

**After:**
```go
err := linkSelfClient.SendMessage(ctx, params.PeerDID, params.Message)
```

### 7. ピアへの接続

**Before:**
```go
_, err := linkSelfNode.Connect(ctx, params.PeerDID)
```

**After:**
```go
err := linkSelfClient.Connect(ctx, params.PeerDID)
```

### 8. ヘルパー関数の削除

以下の関数は`core/pkg/linkself/client.go`に移動したため、削除できます：

- `loadOrGenerateIdentity()`
- `saveIdentity()`

これらの機能は`client.Start()`内で自動的に処理されます。

## 完全な例

### Before

```go
package main

import (
    "context"
    "github.com/SeijiShii/link-self/core/internal/did"
    "github.com/SeijiShii/link-self/core/internal/node"
    "github.com/libp2p/go-libp2p/core/peer"
)

var linkSelfNode *node.Node

func startNode(ctx context.Context, identityPath string, listenAddrs []string, bootstrapPeers []string) error {
    identity, err := loadOrGenerateIdentity(identityPath)
    if err != nil {
        return err
    }
    
    var peers []peer.AddrInfo
    for _, addrStr := range bootstrapPeers {
        info, err := peer.AddrInfoFromString(addrStr)
        if err != nil {
            return err
        }
        peers = append(peers, *info)
    }
    
    cfg := node.Config{
        Identity:       identity,
        ListenAddrs:    listenAddrs,
        BootstrapPeers: peers,
    }
    
    n, err := node.New(ctx, cfg)
    if err != nil {
        return err
    }
    
    if err := n.Start(ctx); err != nil {
        return err
    }
    
    linkSelfNode = n
    return nil
}

func getMyDID() string {
    if linkSelfNode == nil {
        return ""
    }
    return linkSelfNode.Identity.DID
}

func sendMessage(ctx context.Context, peerDID, message string) error {
    if linkSelfNode == nil {
        return errors.New("node not started")
    }
    memberDIDs := []string{linkSelfNode.Identity.DID, peerDID}
    return linkSelfNode.SendToGroup(ctx, memberDIDs, []byte(message))
}
```

### After

```go
package main

import (
    "context"
    "github.com/SeijiShii/link-self/core/pkg/linkself"
)

var linkSelfClient linkself.Client

func startNode(ctx context.Context, identityPath string, listenAddrs []string, bootstrapPeers []string) error {
    client := linkself.NewClient()
    
    config := linkself.Config{
        IdentityPath:  identityPath,
        ListenAddrs:   listenAddrs,
        BootstrapPeers: bootstrapPeers,
    }
    
    _, err := client.Start(ctx, config)
    if err != nil {
        return err
    }
    
    linkSelfClient = client
    return nil
}

func getMyDID() string {
    if linkSelfClient == nil {
        return ""
    }
    return linkSelfClient.GetMyDID()
}

func sendMessage(ctx context.Context, peerDID, message string) error {
    if linkSelfClient == nil {
        return errors.New("node not started")
    }
    return linkSelfClient.SendMessage(ctx, peerDID, message)
}
```

## 注意事項

### 1. 設定の違い

- **BootstrapPeers**: `peer.AddrInfo`のスライスから文字列のスライスに変更されました。パースは`client.Start()`内で自動的に行われます。

### 2. エラーハンドリング

エラーメッセージやエラー型が変更される可能性があります。エラーハンドリングコードを確認してください。

### 3. メッセージハンドラの設定タイミング

`SetOnMessage()`は`Start()`の前または後に呼び出すことができますが、実際にメッセージを受信するにはノードが起動している必要があります。

### 4. ノード情報の取得

`Start()`は`*NodeInfo`を返します。これにはDIDとリスンアドレスが含まれます：

```go
info, err := client.Start(ctx, config)
if err != nil {
    return err
}
fmt.Printf("DID: %s\n", info.DID)
fmt.Printf("ListenAddr: %s\n", info.ListenAddr)
```

## テスト

移行後は、以下のテストを実行して動作を確認してください：

```bash
# Daemonテスト
cd core/cmd/linkself-daemon
go test -v

# 公開APIテスト
cd core/pkg/linkself
go test -v
```

## トラブルシューティング

### エラー: "use of internal package not allowed"

`internal`パッケージを直接インポートしている可能性があります。すべての`internal`インポートを削除し、`core/pkg/linkself`のみを使用してください。

### エラー: "node not started"

`Start()`を呼び出す前に他のメソッドを呼び出している可能性があります。必ず`Start()`を最初に呼び出してください。

### メッセージが受信されない

`SetOnMessage()`が`Start()`の後に呼び出されていることを確認してください。また、ノードが正常に起動していることを確認してください。

## 参考

- [公開APIドキュメント](../../core/pkg/linkself/README.md)
- [結合度分析](coupling-analysis.md)
- [改善計画](coupling-improvement-plan.md)
- [テスト結果](refactoring-test-results.md)
