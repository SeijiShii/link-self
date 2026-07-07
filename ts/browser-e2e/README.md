# @linkself/browser-e2e — 実ブラウザ E2E（Playwright）

`@linkself/core` をヘッドレス Chromium（実ブラウザ）で検証する E2E ハーネス。

> **状態: 4/4 PASS（2026-07-07 初回実行完了）**
> 初回実行で 2 件の実ブラウザ限定バグを検出・修正した（下記「初回実行で判明した点」）。
> Node のユニットテストでは再現しない、実ブラウザでしか出ない問題だったため実行の価値があった。

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

## 初回実行で判明した点（2026-07-07）

実ブラウザでしか出ない 2 件を検出し修正した:

1. **ブラウザは loopback アドレスへの dial を既定で拒否する**（local-network 保護 → `DialDeniedError`）。
   本番はブラウザから公開リレーへ dial するため問題ないが、localhost の Go ノードに繋ぐ E2E では
   libp2p の `connectionGater.denyDialMultiaddr: () => false` で明示的に許可する必要がある。
2. **sqlite OPFS 永続化は Worker でしか動かない**（`@linkself/core` 側の実バグを修正）。
   OPFS SAHPool VFS が要求する `createSyncAccessHandle` はメインスレッドに存在せず（Worker のみ）、
   旧実装はメインスレッドで静かに一時 DB へフォールバックしていた（＝リロードで消える）。
   `SqliteWasmDatabase` を **専用 Worker で SAHPool VFS を動かす RPC 方式**に変更（`src/sqlite-worker.ts`）。
   `:memory:`（Node テスト）はメインスレッドの `oo1.DB` のまま。
   OPFS API 可用性の実測（`opfsProbe`）: `mainThreadSyncAccessHandle=false` / `workerSyncAccessHandle=true`。

## 自動化の範囲（設計メモ）

| 項目 | 本ハーネス | 備考 |
|---|---|---|
| OPFS / sqlite-wasm / Web Locks / WS 接続 | ✅ 自動 | 実 Chromium |
| リレー経由接続（/p2p-circuit） | 追加可能 | `poc-wsnode -relay` を使いシナリオ追加 |
| FastStart / peerstore 永続化 | 追加予定 | 実装後にリロード跨ぎで検証 |
| iOS 実機（ホーム画面追加・Web Push） | ❌ 手動 | Playwright WebKit ≠ iOS Safari |
| 実モバイル回線・実 NAT | ❌ 手動 | flow:release の実機確認で実施 |
