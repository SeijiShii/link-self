# LinkSelf Public API

`core/pkg/linkself`は、LinkSelfコアライブラリの公開APIを提供するパッケージです。

## 概要

このパッケージは、LinkSelfノードの作成、起動、停止、メッセージ送信、接続などの機能を提供します。すべての機能は`Client`インターフェースを通じてアクセスします。

このパッケージを通じて、`internal`パッケージの実装詳細を隠蔽し、安定したインターフェースを提供します。

## ドキュメント

**完全なAPIドキュメントは、godocで確認できます（英語と日本語の両方に対応）：**

### オンラインで確認（推奨）

リポジトリが公開されている場合、**pkg.go.dev**で自動的に公開されます：

**https://pkg.go.dev/github.com/SeijiShii/link-self/core/pkg/linkself**

> **注意**: 初回公開時は、上記URLにアクセスして「Request」ボタンをクリックするか、`go get`コマンドを実行すると、数分でインデックスされます。

### ローカルで確認

#### コマンドライン

```bash
# パッケージ全体
go doc github.com/SeijiShii/link-self/core/pkg/linkself

# 特定の型や関数
go doc linkself.Config
go doc linkself.Client
go doc linkself.NewClient
go doc linkself.Client.Start
```

#### Webブラウザ

```bash
godoc -http=:6060
# ブラウザで http://localhost:6060/pkg/github.com/SeijiShii/link-self/core/pkg/linkself を開く
```

### GitHubリポジトリ

GitHubリポジトリ上でも、godocコメントは通常のコメントとして表示されます：
- `core/pkg/linkself/types.go`
- `core/pkg/linkself/client.go`

**注意**: godocコメントには英語と日本語の両方が含まれています。英語の説明の後に日本語の説明が続きます。

詳細は [DOCUMENTATION.md](DOCUMENTATION.md) を参照してください。

## クイックスタート

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/SeijiShii/link-self/core/pkg/linkself"
)

func main() {
    ctx := context.Background()
    
    // クライアントを作成
    client := linkself.NewClient()
    
    // ノードを起動
    config := linkself.Config{
        ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
    }
    
    info, err := client.Start(ctx, config)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Node started with DID: %s\n", info.DID)
    
    // メッセージハンドラを設定
    client.SetOnMessage(func(peerDID string, payload []byte) {
        fmt.Printf("Received message from %s: %s\n", peerDID, string(payload))
    })
    
    // ノードを停止
    defer client.Stop(ctx)
}
```

## インストール

```go
import "github.com/SeijiShii/link-self/core/pkg/linkself"
```

## 主な型と関数

- **Client**: LinkSelfノードと対話するためのインターフェース
- **NewClient()**: 新しいクライアントを作成
- **Config**: ノードの設定
- **NodeInfo**: 起動したノードの情報
- **MessageHandler**: メッセージ受信時のコールバック関数型

詳細はgodocを参照してください。

## 使用例

### 基本的な使用

```go
client := linkself.NewClient()
info, err := client.Start(ctx, linkself.Config{})
if err != nil {
    return err
}
defer client.Stop(ctx)

did := client.GetMyDID()
fmt.Printf("My DID: %s\n", did)
```

### カスタム設定での起動

```go
config := linkself.Config{
    IdentityPath: "/custom/path/identity.json",
    ListenAddrs: []string{
        "/ip4/0.0.0.0/tcp/4001",
        "/ip6/::/tcp/4001",
    },
    BootstrapPeers: []string{
        "/ip4/127.0.0.1/tcp/4001/p2p/12D3KooWExample",
    },
}

info, err := client.Start(ctx, config)
```

### メッセージの送受信

```go
// メッセージハンドラを設定
client.SetOnMessage(func(peerDID string, payload []byte) {
    fmt.Printf("Received: %s\n", string(payload))
})

// ピアに接続
peerDID := "did:key:z2DgdjLjzH9DhxtgjCjrxoAooaFNMgEJMBuRY6ddm8M3sfp"
if err := client.Connect(ctx, peerDID); err != nil {
    log.Fatal(err)
}

// メッセージを送信
if err := client.SendMessage(ctx, peerDID, "Hello, World!"); err != nil {
    log.Fatal(err)
}
```

## アーキテクチャ

このパッケージは、`internal`パッケージへの依存を集約し、安定した公開APIを提供します：

```
Application/Daemon
    ↓
pkg/linkself (公開API)
    ↓
internal/* (実装詳細)
```

この設計により：
- **疎結合**: アプリケーションは`internal`パッケージに直接依存しない
- **変更耐性**: `internal`の変更がアプリケーションに影響しない
- **再利用性**: 複数のアプリケーションが同じAPIを使用可能
- **テスト容易性**: インターフェースなのでモック可能

## テスト

このパッケージには包括的なユニットテストが含まれています。テストを実行するには：

```bash
cd core/pkg/linkself
go test -v
```

## 関連ドキュメント

- [結合度分析](../../chat-client/docs/coupling-analysis.md)
- [改善計画](../../chat-client/docs/coupling-improvement-plan.md)
- [移行ガイド](../../chat-client/docs/migration-guide.md)
- [テスト結果](../../chat-client/docs/refactoring-test-results.md)
