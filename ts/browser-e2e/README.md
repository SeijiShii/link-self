# @linkself/browser-e2e — 実ブラウザ E2E（Playwright）

`@linkself/core` をヘッドレス Chromium（実ブラウザ）で検証する E2E ハーネス。

> **状態: 未実行（2026-07-07 実装のみ・実行保留）**
> 他セッションの Playwright とのリソース競合を避けるため、初回実行は保留中。
> 下記の手順で実行し、結果を本 README と `docs/spec/browser-pwa-support.md` §7 に反映すること。

## 検証シナリオ（tests/browser.spec.ts）

1. **cross-origin isolation** — COOP/COEP ヘッダ配信下で `crossOriginIsolated === true`（sqlite-wasm OPFS VFS の前提）
2. **ブラウザ → Go ノード P2P** — WebSocket 接続 + LinkSelf auth + echo 往復（Go ハーネス `core/cmd/poc-wsnode` を自動起動）
3. **OPFS 永続化** — sqlite-wasm に書き込み → **ページリロード** → データ残存
4. **Web Locks 多タブ直列化** — 同一オリジンの 2 タブで同名ロックが直列に取得される

## 実行方法

```bash
cd ts/browser-e2e
npm install
npx playwright install chromium   # 初回のみ（ブラウザバイナリ取得）
npm test                          # vite dev server + Go ハーネスは自動起動
```

- リソース節約のため `workers: 1`（Chromium 1 プロセス）に設定済み
- WSL2 で Chromium の共有ライブラリ不足が出る場合: `npx playwright install --with-deps chromium`（sudo 必要）

## 自動化の範囲（設計メモ）

| 項目 | 本ハーネス | 備考 |
|---|---|---|
| OPFS / sqlite-wasm / Web Locks / WS 接続 | ✅ 自動 | 実 Chromium |
| リレー経由接続（/p2p-circuit） | 追加可能 | `poc-wsnode -relay` を使いシナリオ追加 |
| FastStart / peerstore 永続化 | 追加予定 | 実装後にリロード跨ぎで検証 |
| iOS 実機（ホーム画面追加・Web Push） | ❌ 手動 | Playwright WebKit ≠ iOS Safari |
| 実モバイル回線・実 NAT | ❌ 手動 | flow:release の実機確認で実施 |
