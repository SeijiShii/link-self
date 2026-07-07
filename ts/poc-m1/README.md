# M1 疎通 PoC — js-libp2p ↔ Go LinkSelf ノード

ブラウザ / PWA 対応（`docs/spec/browser-pwa-support.md`）のマイルストーン M1。
js-libp2p（TypeScript）から Go 実装の LinkSelf ノードへ接続し、ワイヤレベルの互換を実証する。

## 実証内容（2026-07-07 PASSED）

| レイヤ | 検証結果 |
|---|---|
| トランスポート | WebSocket（`/ip4/…/tcp/…/ws`）直結、および Circuit Relay v2 経由（`/p2p-circuit`）の両方で接続成立 |
| セキュリティ | Noise ハンドシェイク成立（js: `@chainsafe/libp2p-noise` ↔ go-libp2p 標準） |
| 多重化 | yamux 成立 |
| プロトコル | `/linkself/msg/1.0.0`（uint32 BE 長プレフィックス framing）を双方向に交換 |
| DID | Go 側が js ピアの Ed25519 鍵から `did:key` を導出できることを確認（認証層互換の土台） |

## 実行方法

```bash
cd ts/poc-m1
npm install
bash run.sh
```

`run.sh` は Go 側ハーネス（`core/cmd/poc-wsnode`）をビルド・起動し（echo ノード + リレー + リレー経由でしか届かない leaf ノード）、TS クライアント（`src/poc.ts`）が両経路の echo 往復を検証して exit code で結果を返す。

## 実装ノート

- リレー経由の接続は libp2p 上「limited connection」扱い。ストリーム開設には明示許可が必要:
  - Go 側: `network.WithAllowLimitedConn`（`internal/node` で対応済み）
  - js 側: `dialProtocol` / `handle` の `runOnLimitedConnection: true`
- `@libp2p/websockets` v10+ では旧 `filters.all` は廃止済み。`/ip4/…/ws` へのダイヤルはデフォルトで可能
- js 側は listen アドレスを持たない（ブラウザと同条件）。Go 側からの echo 返信は既存接続上の新規ストリームで届く

## 次のステップ（M2）

LinkSelf 固有プロトコル層（auth チャレンジレスポンス / envelope / syncdb / group / …）の TS 再実装。
本 PoC の framing 実装（`encodeFrame` / `readFrame`）がその出発点になる。
