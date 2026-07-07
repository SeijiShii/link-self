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
| `src/router.ts` | `internal/node/router.go` | ✅ | 単体テスト（envelope 種別ルーティング） |
| `src/storeforward.ts` | `internal/storeforward` | ✅ | 単体テスト + live interop（未接続時キュー→接続時フラッシュ） |
| `src/node.ts`（`LinkSelfNode`） | `internal/node`（リーフ側） | ✅ | **live interop**: connectToAddr（dial+auth+flush）・sendMessage・受信ルーティングを Go ノード相手に確認 |
| `src/syncdb.ts` | `internal/syncdb` | ✅ | SyncRecord JSON が golden ベクタでバイト一致（`body:null` 含む）。LWW 適用は Go の sync_test.go を移植 |
| `src/role.ts` / `src/permission.ts` | `internal/role` / `internal/permission` | ✅ | DAG 循環検出・上位ロール包含・self/owner 委譲を単体テストで固定化 |
| `src/group.ts` | `internal/group` | ✅ | 解散・自動オーナー昇格・他オーナー降格不可などのドメイン規則を移植 |
| `src/groupshare.ts` | `internal/groupshare` | ✅ | SharedRecord / SubAnnouncement JSON golden 一致。チャンネル・保持期間・ロール権限・**topic 購読フィルタリング**込み |
| `src/devicesync.ts` | `internal/devicesync` | ✅ | ChangeEntry JSON golden 一致。LWW レプリケーション・差分同期／full dump フォールバック・ChangeLog 保持ポリシー |
| `src/sqlproxy.ts` | `internal/sqlproxy` | ✅ | 書き込み検出・テーブル名抽出・マイグレーション（Go と同一挙動、テーブル名抽出の限界もパリティ維持）。実 DB は `SqlDatabase` 抽象の背後（M3 で sqlite-wasm を注入） |
| network / pairing / dht / dataroot / storage | `internal/*` | ❌ 未着手 | network（NetworkAPI）と pairing は次工程。dht はブラウザではクライアントモード限定、dataroot/storage は M3（OPFS）の領域 |
| Client 組み立て（`pkg/linkself` 相当） | `pkg/linkself` | ❌ 未着手 | 上記モジュールを束ねる公開 API。M2 の最終工程 |

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

1. network（NetworkAPI: ネットワーク作成・招待・脱退）/ pairing（QR 端末ペアリング）
2. Client 組み立て（`pkg/linkself` 相当の公開 API — node + syncdb + groupshare + devicesync を束ねる）
3. Go ノードとの groupshare / devicesync live interop テスト
4. sqlite-wasm + OPFS 上の MyDB 相当 API（M3、`SqlDatabase` 抽象へ注入）
