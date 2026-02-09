# ドキュメントの公開方法

このドキュメントでは、`core/pkg/linkself`パッケージのgodocを公開する方法を説明します。

## 1. pkg.go.dev（推奨・自動公開）

**pkg.go.dev**は、Goの公式パッケージドキュメントサイトです。GitHubリポジトリが公開されている場合、**自動的にインデックス**されます。

### 公開方法

1. **GitHubリポジトリを公開**（既に公開されている場合、このステップは不要）
2. **Gitタグでバージョンをリリース**
   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```
3. **go getでパッケージを取得**（pkg.go.devが自動的にインデックス）
   ```bash
   go get github.com/SeijiShii/link-self/core/pkg/linkself@v0.1.0
   ```

### アクセスURL

公開後、以下のURLでアクセスできます：

```
https://pkg.go.dev/github.com/SeijiShii/link-self/core/pkg/linkself
```

### 手動リクエスト（必要に応じて）

自動的にインデックスされない場合、以下の方法で手動リクエストできます：

1. ブラウザで `https://pkg.go.dev/github.com/SeijiShii/link-self/core/pkg/linkself` にアクセス
2. 「Request」ボタンをクリック
3. または、`go get`コマンドを実行：
   ```bash
   go get github.com/SeijiShii/link-self/core/pkg/linkself@latest
   ```

### メリット

- ✅ **自動更新**: 新しいバージョンがリリースされると自動的に更新される
- ✅ **検索可能**: Go開発者がパッケージを検索できる
- ✅ **標準的**: Goコミュニティで標準的な方法
- ✅ **無料**: 完全に無料で利用可能
- ✅ **日本語対応**: godocコメントに日本語が含まれていれば、そのまま表示される

## 2. GitHubリポジトリでの閲覧

GitHubリポジトリ上でも、godocコメントは**通常のコメントとして表示**されます。

### 閲覧方法

1. GitHubリポジトリで `core/pkg/linkself/types.go` や `core/pkg/linkself/client.go` を開く
2. コメントがそのまま表示される

### メリット

- ✅ **コードと一緒に閲覧**: コードとドキュメントを同時に確認できる
- ✅ **Git履歴**: ドキュメントの変更履歴も追跡できる
- ✅ **PRレビュー**: プルリクエストでドキュメントの変更もレビューできる

### デメリット

- ❌ **godoc形式ではない**: 通常のコメントとして表示されるため、godocの整形された形式ではない
- ❌ **検索機能なし**: pkg.go.devのような検索機能はない

## 3. ローカルでの閲覧

開発中は、ローカルでgodocを確認できます。

### コマンドライン

```bash
# パッケージ全体
go doc github.com/SeijiShii/link-self/core/pkg/linkself

# 特定の型や関数
go doc linkself.Config
go doc linkself.Client
go doc linkself.NewClient
```

### Webサーバー

```bash
# godocサーバーを起動
godoc -http=:6060

# ブラウザで開く
# http://localhost:6060/pkg/github.com/SeijiShii/link-self/core/pkg/linkself
```

## 4. READMEへのリンク追加

リポジトリのREADMEに、pkg.go.devへのリンクを追加することを推奨します。

### 例

```markdown
## ドキュメント

- **APIドキュメント**: https://pkg.go.dev/github.com/SeijiShii/link-self/core/pkg/linkself
- **ローカルで確認**: `go doc github.com/SeijiShii/link-self/core/pkg/linkself`
```

## 5. GitHub Pages（オプション）

GitHub Pagesでgodocをホストすることも可能ですが、**pkg.go.devの方が推奨**されます。

### 設定方法（参考）

```bash
# godocを静的HTMLとして生成
godoc -url /pkg/github.com/SeijiShii/link-self/core/pkg/linkself > docs.html

# GitHub Pagesにデプロイ
```

### デメリット

- ❌ **手動更新が必要**: 新しいバージョンごとに手動で更新する必要がある
- ❌ **メンテナンス負荷**: pkg.go.devと比べてメンテナンスが大変

## 推奨される方法

1. **pkg.go.devを使用**（推奨）
   - 自動的にインデックスされる
   - Goコミュニティで標準的
   - 無料で利用可能

2. **READMEにリンクを追加**
   - pkg.go.devへのリンクをREADMEに追加
   - 開発者が簡単にアクセスできるようにする

3. **GitHubリポジトリも活用**
   - コードレビュー時にコメントを確認
   - ドキュメントの変更履歴を追跡

## 現在の状態

現在のモジュールパス: `github.com/SeijiShii/link-self/core`

公開後、以下のURLでアクセス可能になります：
- https://pkg.go.dev/github.com/SeijiShii/link-self/core/pkg/linkself

## 参考リンク

- [pkg.go.dev](https://pkg.go.dev/)
- [Go Modules ドキュメント](https://go.dev/ref/mod)
- [godoc コマンド](https://pkg.go.dev/golang.org/x/tools/cmd/godoc)
