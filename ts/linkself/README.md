# @linkself/core — LinkSelf TypeScript 実装

Go 実装（`core/`）と**ワイヤ互換の第二実装**。ブラウザ / PWA から LinkSelf ネットワークに
リーフノードとして参加するためのプロトコル層を TypeScript で再実装する
（背景: `docs/spec/browser-pwa-support.md`）。

## 実装状況（M2 進行中）

| モジュール | 対応する Go パッケージ | 状態 | 検証 |
|---|---|---|---|
| `src/did.ts` | `internal/did` | ✅ | golden ベクタで DID / PeerId / DHT キーが Go とバイト一致 |
| `src/framing.ts` | `internal/node`（uint32 BE framing） | ✅ | M1 PoC + interop テスト |
| `src/envelope.ts` | `internal/envelope` | ✅ | golden ベクタで JSON バイト一致 |
| `src/auth.ts` | `internal/auth` | ✅ | **live interop**: js→Go / Go→js 双方向の認証成功 |
| storeforward / syncdb / group / role / permission / devicesync / sqlproxy 等 | `internal/*` | ❌ 未着手 | — |

## 注意（Go 実装との互換性のための仕様）

- **did:key の multicodec は単バイト `0xed`**。標準の varint（`0xed 0x01`）ではない。
  Go 実装（`internal/did`）に合わせた挙動であり、「標準に修正」してはいけない
- envelope の JSON は Go の `encoding/json` 出力（`[]byte` → パディング付き標準 base64、キー順 `type`,`payload`）にバイト一致させている
- auth の署名対象はチャレンジ 32 バイトそのもの（プレフィックスなし）。Ed25519 は決定的署名なので golden ベクタで固定化済み

## テスト

```bash
cd ts/linkself
npm install
npm test          # 単体 + interop（go が PATH にあれば Go ノードを起動して実施）
npm run typecheck
```

golden ベクタ（`test/vectors.ts`）は Go 実装から生成した決定値。Go 側のワイヤ形式を
変更した場合は再生成する（シード `0102…20` の Ed25519 鍵）。

## 次のステップ

1. store-and-forward / メッセージルーティング（`internal/node` の router 相当）
2. syncdb（差分同期）— 工数の本丸
3. group / role / permission
4. sqlite-wasm + OPFS 上の MyDB 相当 API（M3）
