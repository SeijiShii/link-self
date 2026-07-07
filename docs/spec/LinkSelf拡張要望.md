# LinkSelf 拡張要望（ブラウザ / PWA 対応）

Home Visit Suite を単一 PWA（2026-07-07 全面 PWA 化決定）として実装するために、LinkSelf 側に必要となる機能追加・変更事項をまとめる。

本ドキュメントは **LinkSelf リポジトリ（`github.com/SeijiShii/link-self`）への提案書**の原稿であり、LinkSelf 側の仕様書・ロードマップに取り込む。

- 対応する LinkSelf 側設計文書: `link-self/docs/spec/browser-pwa-support.md`（2026-07-02 の設計検討をコミット済み。技術的実現性・トポロジ・ホスティング検討の SoT）
- 関連する既存仕様: `link-self/docs/spec/mobile-support.md`（旧モバイルネイティブ対応。共通要件はブラウザ対応へ引き継ぐ）

## 旧版（モバイル gomobile 対応要望）の扱い

- 本ドキュメントの旧版は Expo + gomobile によるモバイルネイティブ対応の要望だったが、**2026-07-07 の全面 PWA 化決定により gomobile ルートは廃止**した
- 旧版の内容は `link-self/docs/spec/mobile-support.md` に転記済みであり、履歴はそちらに残る
- 旧版の要求のうち以下は**ブラウザ対応でもそのまま必要**であり、本ドキュメントに引き継ぐ:
  - 高速起動（FastStart / KnownPeerHints）
  - peerstore / routing table の永続化
  - Circuit Relay v2 クライアント対応
  - 前景起動 → 高速差分同期 → graceful stop のライフサイクル
  - 差分同期の優先度制御

---

## 1. 背景

- Home Visit Suite は**単一 PWA + ロール別 UI** の 1 系統に統合する（管理・編集・活動の全ロールが同一アプリを使う）
- データ永続化・同期は引き続き LinkSelf を基盤とするが、ブラウザでは Go コアを動かせないため **TypeScript 実装が必要**
- Go コアの WASM 化は不可（go-libp2p のトランスポート層が生ソケット前提で、ブラウザに API が存在しない。詳細は `browser-pwa-support.md` §2.1）
- 方針: **js-libp2p 上に LinkSelf プロトコル層を TS で再実装**する（ワイヤ互換の「第二実装」。Go ノードと相互運用する）

## 2. 前提制約（ブラウザ環境）

| 制約 | 内容 |
|------|------|
| トランスポート | TCP / QUIC 不可。WebSocket / WebTransport / WebRTC のみ |
| 着信 | ブラウザピアは外部から接続を受けられない → Circuit Relay v2 が必須 |
| DHT | クライアントモードのみ（フル参加不可） |
| ローカル発見 | mDNS / Bluetooth 不可 |
| バックグラウンド | 実行ゼロ。タブ / PWA を閉じると完全停止 |
| 通知 | Web Push はプッシュサーバー経由が仕様上必須 |
| TLS 証明書 | WebTransport / WebRTC-direct の certhash 方式により**ドメイン・CA 証明書なしで接続可能** |

### Home Visit Suite のユースケース上の前提

- 活動メンバーはチェックアウト中（訪問中）にアプリを前景で操作する
- バックグラウンドでの常時同期は**不要**（次回起動時の差分同期で十分）
- したがって「前景起動時に高速に同期完了 → 非表示遷移で graceful stop」のライフサイクルで足りる

## 3. 要求事項

優先度: **必須** = PWA 対応の前提として必要、**推奨** = UX 上望ましい、**将来** = 次フェーズ以降。

### 3.1 必須

#### 3.1.1 LinkSelf TypeScript 実装（ブラウザ版クライアント）

- js-libp2p + sqlite-wasm (OPFS) + WebCrypto を土台に、`core/internal` の LinkSelf 固有プロトコル層（auth / envelope / storeforward / syncdb / group / role / permission / devicesync / sqlproxy 等 約 17 パッケージ相当）を TS で再実装する
- libp2p のプロトコル ID とメッセージ形式を Go 実装と揃え、**Go ノードと相互運用可能**にする
- アプリ向け API 面は Go 版 `pkg/linkself`（`Client` / `MyDB` / `NetworkAPI` / `SharedDB`）と同等の概念を TS で提供する
- 複数タブからの同時アクセスは単一アクセサに直列化する（Web Locks / SharedWorker 等、方式は実装側で決定）

#### 3.1.2 Go デーモンのブラウザ向けトランスポート

- 常時稼働ノード（Go デーモン）に `/ws` または `/webtransport` の listen アドレスを追加する（go-libp2p は対応済みのため Config 追加程度の想定）
- certhash 方式（WebTransport / WebRTC-direct）を採用し、ドメイン取得・CA 証明書なしで https 配信の PWA から接続できるようにする

#### 3.1.3 常時稼働ノード（リレー / ブートストラップ / メールボックス）

- ブラウザピアの通信を成立させる 3 役を Go デーモンが担えるようにする:
  - **Circuit Relay v2**（リレー）: ブラウザ間接続の中継。E2E 暗号化により中身は読めない
  - **ブートストラップ**: ブラウザ向けトランスポートを話す入口ノード
  - **メールボックス（store-and-forward）**: オフラインピア宛メッセージの預かり
- `Config.BootstrapPeers` / `Config.CircuitRelays` は**複数登録できる**リスト形式とし、稼働率を多重化で担保できる設計にする

#### 3.1.4 高速起動と peerstore 永続化（mobile-support §3.1.3 / §3.1.4 の引き継ぎ）

- `FastStart`: 起動時の DHT full bootstrap をスキップし、前回保存の peerstore を起点に既知ピアへ直接接続
- `KnownPeerHints []string`: 既知ピアの multiaddr を外部から注入可能に
- TS 実装では peerstore / routing table を OPFS / IndexedDB に永続化し、次回起動時に復元する
- 理由: PWA を開いてから同期結果が見えるまでの体感待ち時間の短縮。バックグラウンド実行ゼロのブラウザでは毎回コールドスタートになるため、モバイル以上に重要

### 3.2 推奨

#### 3.2.1 WebRTC ブラウザ間直結

- リレー経由をフォールバックとし、シグナリング後は WebRTC でブラウザ同士が直接通信するトポロジ（`browser-pwa-support.md` §3 パターン 2）
- 必須ではない（リレー経由で機能は成立する）が、リレーの帯域負荷を下げ、レイテンシを改善する

#### 3.2.2 差分同期の優先度制御（mobile-support §3.2.2 の引き継ぎ）

- 前景起動時の短時間で**ユーザーに見える範囲**のデータを先に同期できることが UX 上重要
- 「次回同期バッチで優先するテーブル / チャンネル集合」のヒントを受け付ける API（例: `SyncPreferTables` / `SyncPreferChannels`）

### 3.3 将来（本スコープ外・検討事項として記載）

#### 3.3.1 Web Push 代理ノード

- Web Push はプッシュサーバー経由が仕様上必須のため、**代理ノード（常時稼働ノード）が他メンバーのデータ変更を検知し Web Push を送出**する構成が取り得る（mobile-support §3.3.1 の APNs/FCM 問題と同型）
- iOS では 16.4+ かつホーム画面追加時のみ Web Push が有効という制約がある
- 本スコープでは「通知は画面を開いた時に差分同期で表示」方針とし、プッシュ通知は将来検討

## 4. マイルストーン案

| フェーズ | 内容 | 成果物 |
|---------|------|--------|
| M1: 疎通 PoC | js-libp2p ↔ go-libp2p を WebSocket + Circuit Relay v2 で接続し、Noise ハンドシェイクと基本メッセージ交換を確認 | PoC コード・接続測定値 |
| M2: プロトコル層 TS 再実装 | auth / envelope / syncdb 等の LinkSelf 固有プロトコルを TS 化し、Go ノードと相互同期 | linkself-ts（仮）初版 |
| M3: ストレージ層 | sqlite-wasm + OPFS 上の `MyDB` 相当 API、多タブ直列化 | TS 版 MyDB |
| M4: 常時稼働ノード構成 | Go デーモンの /ws・relay・メールボックス構成、複数ノード登録 | デーモン設定・運用手順 |
| M5: PWA 統合 | home-visit-suite の PWA から TS 実装を利用、FastStart・peerstore 永続化込みで前景同期時間を測定 | 統合アプリ・測定値 |

## 5. 参考

- `link-self/docs/spec/browser-pwa-support.md` — 実現方式・トポロジ・ホスティング検討（Vercel 不可 / Oracle Cloud Always Free 推奨 / ローカル Windows ノードの条件）の詳細
- js-libp2p — https://github.com/libp2p/js-libp2p
- sqlite-wasm — https://sqlite.org/wasm/doc/trunk/index.md

## 6. 本ドキュメントの扱い

- **本ドキュメントは home-visit-suite 側の要望をまとめた原稿**であり、LinkSelf 側の正式仕様になるわけではない
- LinkSelf 側で実装スコープ・優先度・API 署名が確定したら、home-visit-suite 側は**本ドキュメントを削除するか、LinkSelf 側仕様への参照のみに縮退させる**
