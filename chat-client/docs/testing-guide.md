# テスト動作ガイド

このドキュメントでは、LinkSelf Chat Clientのテスト動作方法を説明します。

## 前提条件

### 必要なソフトウェア

1. **Node.js** 18以上
   ```bash
   node --version  # v18以上であることを確認
   ```

2. **npm** または **yarn**
   ```bash
   npm --version
   ```

3. **Go** 1.24.6以上（daemonビルド用、coreモジュールの要件）
   ```bash
   go version  # go1.24.6以上であることを確認
   ```
   
   **注意**: `core`モジュールが`go 1.24.6`を要求するため、Goのバージョンが古い場合はアップグレードが必要です。

4. **Windows環境**（この段階ではWindowsを優先）

## セットアップ手順

### 1. 依存関係のインストール

```bash
cd chat-client
npm install
```

これにより、以下のパッケージがインストールされます：
- Electron
- React
- TypeScript
- Vite
- その他の開発依存関係

### 2. LinkSelf Daemonのビルド

```bash
# Windows用にビルド
npm run build:daemon
```

このコマンドは以下を実行します：
- `core/cmd/linkself-daemon/main.go` をコンパイル
- `build/linkself-daemon.exe` を生成

**注意**: daemonは`core`モジュール内に配置されているため、`core`モジュールの`internal`パッケージにアクセスできます。

**注意**: ビルドが成功すると、`build/` フォルダに `linkself-daemon.exe` が作成されます。

### 3. TypeScriptのコンパイル（オプション）

開発モードでは自動コンパイルされますが、事前にコンパイルする場合：

```bash
npm run build
```

## テスト動作方法

### 方法1: 開発モード（推奨）

開発モードでは、Viteの開発サーバーとElectronが同時に起動します：

```bash
npm run dev
```

このコマンドは以下を実行します：
1. Vite開発サーバーを起動（`http://localhost:5173`）
2. サーバーが起動するまで待機
3. Electronアプリを起動し、開発サーバーに接続

**動作確認ポイント**:
- Electronウィンドウが開く
- ヘッダーに「LinkSelf Chat」が表示される
- 自分のDIDがヘッダーに表示される（起動後数秒）
- 開発者ツールが自動的に開く（デバッグ用）

### 方法2: 本番ビルド後の実行

1. **ビルド**:
   ```bash
   npm run build
   npm run build:daemon
   ```

2. **Electron起動**:
   ```bash
   npm run electron
   ```

## 動作確認チェックリスト

### 初期起動時

- [ ] Electronウィンドウが正常に開く
- [ ] アプリのタイトル「LinkSelf Chat」が表示される
- [ ] ヘッダーが緑色（LINEライクな色）で表示される
- [ ] 数秒後に自分のDIDがヘッダーに表示される
- [ ] 連絡先リストが空の状態で表示される
- [ ] チャットウィンドウに「連絡先を選択してチャットを開始」が表示される

### UI確認

- [ ] 連絡先リストのレイアウトが正しく表示される
- [ ] チャットウィンドウのレイアウトが正しく表示される
- [ ] メッセージ入力欄が表示される
- [ ] 送信ボタンが表示される（初期状態では無効）

### 機能確認（モック段階）

現在はモック表示までなので、以下の機能は後続フェーズで実装予定：

- [ ] 連絡先の追加機能
- [ ] メッセージの送信機能
- [ ] メッセージの受信機能
- [ ] P2P通信機能

## トラブルシューティング

### 問題1: `npm install` が失敗する

**原因**: Node.jsのバージョンが古い、またはネットワーク問題

**解決方法**:
```bash
# Node.jsのバージョン確認
node --version

# npmキャッシュのクリア
npm cache clean --force

# 再度インストール
npm install
```

### 問題2: `npm run build:daemon` が失敗する

**原因**: Goがインストールされていない、バージョンが古い、またはパスが通っていない

**解決方法**:

1. **Goのバージョン確認**:
   ```bash
   go version  # go1.24.6以上が必要
   ```

2. **Goのバージョンが古い場合**:
   - Go 1.24.6以上にアップグレードが必要です
   - [Go公式サイト](https://go.dev/dl/)から最新版をダウンロード
   - または、`go install golang.org/dl/go1.24.6@latest` で特定バージョンをインストール

3. **Goモジュールのダウンロード**:
   ```bash
   cd daemon
   go mod tidy
   ```

4. **手動ビルド**:
   ```bash
   go build -o ../build/linkself-daemon.exe main.go
   ```

**エラーメッセージ例**:
```
go: module ../../core requires go >= 1.24.6 (running go 1.22.2)
```
この場合は、Goのバージョンを1.24.6以上にアップグレードしてください。

### 問題3: Electronが起動しない

**原因**: daemonが見つからない、またはパスの問題

**解決方法**:
1. `build/linkself-daemon.exe` が存在するか確認
2. 存在しない場合は `npm run build:daemon` を実行
3. エラーメッセージを確認（開発者ツールのコンソール）

### 問題4: Viteサーバーが起動しない

**原因**: ポート5173が既に使用されている

**解決方法**:
```bash
# ポートを使用しているプロセスを確認（Windows）
netstat -ano | findstr :5173

# または、vite.config.tsでポートを変更
```

### 問題5: DIDが表示されない

**原因**: daemonの起動に失敗している

**解決方法**:
1. 開発者ツールのコンソールでエラーを確認
2. `build/linkself-daemon.exe` が存在するか確認
3. daemonのログを確認（stderr出力）

## デバッグ方法

### 開発者ツールの使用

開発モードでは自動的に開発者ツールが開きます。以下を確認できます：

1. **Console**: JavaScriptエラーやログ
2. **Network**: HTTPリクエスト（Vite開発サーバーへの接続）
3. **Elements**: DOM構造の確認

### ログの確認

- **Electronメインプロセス**: ターミナルに出力される
- **Reactコンポーネント**: `console.log()` で開発者ツールのコンソールに出力
- **LinkSelf Daemon**: stderrに出力される（ターミナルで確認）

### 手動テスト

daemonを直接テストする場合：

```bash
# daemonを直接起動（stdio経由でJSON-RPCを送信）
cd build
./linkself-daemon.exe

# 別のターミナルからJSON-RPCリクエストを送信（例）
echo '{"jsonrpc":"2.0","method":"start","params":{},"id":1}' | ./linkself-daemon.exe
```

## 次のステップ

モック表示が確認できたら、以下の機能を実装します：

1. 連絡先の追加機能
2. メッセージの送受信機能
3. P2P通信の統合
4. エラーハンドリングの強化
5. UI/UXの改善

## 参考情報

- [Electron公式ドキュメント](https://www.electronjs.org/docs)
- [React公式ドキュメント](https://react.dev/)
- [Vite公式ドキュメント](https://vitejs.dev/)
- [LinkSelf Core API](../../core/README.md)
