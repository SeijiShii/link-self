# Linux環境でのトラブルシューティング

## 問題: GUIウィンドウが真っ白になる

### 症状
- Electronアプリが起動するが、ウィンドウが真っ白
- GPU関連のエラーが表示される（`Failed to send GpuControl.CreateCommandBuffer`）
- ブラウザでは`http://localhost:5173`が正常に表示される

### 原因
Linux環境では、GPUアクセラレーションやVite開発サーバーへの接続に問題が発生することがあります。

### 解決方法

#### 1. GPUアクセラレーションの無効化（既に実装済み）

`src/main.ts`で以下の設定が既に追加されています：

```typescript
// Disable hardware acceleration for Linux compatibility
app.disableHardwareAcceleration();
```

#### 2. Viteサーバーの設定確認

`vite.config.ts`で以下の設定を確認：

```typescript
server: {
  port: 5173,
  host: '0.0.0.0', // Electronからの接続を許可
  strictPort: false,
  cors: true,
}
```

#### 3. 手動でのデバッグ

1. **Viteサーバーが起動しているか確認**:
   ```bash
   curl http://localhost:5173
   ```

2. **Electronの開発者ツールで確認**:
   - 開発者ツールが自動的に開きます
   - Consoleタブでエラーを確認
   - Networkタブで`http://localhost:5173`へのリクエストを確認

3. **環境変数でのデバッグ**:
   ```bash
   DEBUG=* npm run dev
   ```

#### 4. 代替方法: 本番ビルドで確認

開発モードで問題が続く場合、本番ビルドで確認：

```bash
npm run build
npm run electron
```

#### 5. Electronの起動オプション

環境変数でElectronの動作を調整：

```bash
# GPUアクセラレーションを無効化（既にコードで設定済み）
ELECTRON_DISABLE_SANDBOX=1 npm run dev

# または、より詳細なログ
ELECTRON_ENABLE_LOGGING=1 npm run dev
```

### 追加の確認事項

1. **ポート5173が使用可能か確認**:
   ```bash
   netstat -tuln | grep 5173
   # または
   lsof -i :5173
   ```

2. **ファイアウォールの確認**:
   - ローカルホストへの接続がブロックされていないか確認

3. **Electronのバージョン確認**:
   ```bash
   npx electron --version
   ```

4. **システムのログ確認**:
   ```bash
   journalctl -f
   # Electronアプリを起動して、システムレベルのエラーを確認
   ```

### よくあるエラーと対処法

#### `ERR_CONNECTION_REFUSED`
- Viteサーバーが起動していない可能性
- `npm run dev`でViteサーバーが正常に起動しているか確認

#### `ERR_BLOCKED_BY_CLIENT`
- セキュリティポリシーや拡張機能によるブロック
- Electronの`webSecurity`設定を確認

#### GPU関連のエラー
- `app.disableHardwareAcceleration()`が正しく実行されているか確認
- `app.whenReady()`の前に呼び出す必要があります

### 参考リンク

- [Electron Linux Issues](https://www.electronjs.org/docs/latest/tutorial/linux-issues)
- [Vite Server Options](https://vitejs.dev/config/server-options.html)
