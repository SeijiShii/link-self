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
| `src/network.ts` | `internal/network` | ✅ | ロールゲート付き NetworkService。最終メンバー脱退で削除等のドメイン規則 |
| `src/pairing.ts` | `internal/pairing` | ✅ | 鍵転送は libp2p protobuf 形式で **Go の MarshalPrivateKey と golden 一致**（Go 端末 ⇔ js 端末ペアリング互換） |
| `src/invitation.ts` | （Go 未実装・TS 先行） | ✅ | ネットワーク招待トークン（別 DID を参加させる署名付き capability、鍵は運ばない）。create/verify（署名＋期限）・base64url 符号化・`#/join?i=` URL。設計: `docs/spec/network-invitation.md` |
| `src/client.ts`（`LinkSelfClient`） | `pkg/linkself` | ✅ | 全レイヤの組み立て + envelope 配線。**e2e**: 実 libp2p 2 ノードの TS クライアント同士で、オフライン購読→auth 時フラッシュ→topic フィルタ配信→LWW 適用のフルフローを検証。DeviceSyncSubscriptionStore 込み |
| `src/sqlite.ts`（`SqliteWasmDatabase`）+ `src/sqlite-worker.ts` | `storage/sqlite`（ncruces/go-sqlite3 相当） | ✅ 実ブラウザ検証済み | @sqlite.org/sqlite-wasm。**永続（ファイル名指定）は専用 Worker で OPFS SAHPool VFS を動かす**（`createSyncAccessHandle` は Worker のみのため main thread 不可）。`:memory:`（Node テスト）は main thread の `oo1.DB`。多タブ直列化はアプリ層（Web Locks 等）の責務。ブラウザでのリロード跨ぎ永続を `ts/browser-e2e` で検証 |
| `src/mydb.ts`（`MyDB` + `wireSqlSync`） | `pkg/linkself`（myDB） | ✅ | KV（devicesync 経由レプリケーション）+ SQL（sqlproxy 経由）。SQL 書き込みの devicesync ミラーリング（row-readback）は Go client と同一配線。SyncScope は Go 同様 Phase C 待ちの記録のみ |
| dht / dataroot | `internal/*` | ❌ 未着手 | dht はブラウザではクライアントモード限定（リレー経由接続が主経路のため優先度低）。dataroot 相当の OPFS ディレクトリレイアウトは PWA 統合時に決定 |

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

## FastStart / peerstore 永続化（実装済み）

mobile-support §3.1.3 / §3.1.4 のブラウザ版:

- `LinkSelfClientOptions.knownPeers` — start() 時に既知ピアへ並列 dial + 認証（到達不能はスキップ、best-effort）。Go の `FastStart` + `KnownPeerHints` 相当
- `client.snapshotKnownPeers()` — 現在の接続を `{did, addrs[]}` として取得。アプリが localStorage / OPFS に保存し、次回起動時に `knownPeers` へ渡す
- libp2p の `datastore` オプションに永続ストア（ブラウザ: `datastore-idb` = IndexedDB）を渡せば peerstore（アドレス帳）がインスタンスを跨いで残り、**PeerId だけで再 dial 可能**になる（テストで検証済み）

## 次のステップ

1. **ブラウザ実機 E2E は 4/4 PASS 済み**（`ts/browser-e2e/`）。次は FastStart のリロード跨ぎシナリオを追加する
2. dataroot 相当の OPFS ディレクトリレイアウト（`<DID>/suites/<SuiteID>/data.db` 相当）
3. home-visit-suite PWA への統合（M5）。Go 側 Phase C（groupshare ⇔ MyDB sync scope 統合）が入り次第、interop ハーネスを pkg/linkself ベースへ置換
