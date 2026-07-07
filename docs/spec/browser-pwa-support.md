# ブラウザ / PWA 対応検討メモ

LinkSelf をブラウザ内（TypeScript ベース）で動作させ PWA に組み込めるか、および常時稼働ノードの配置をどう設計するかの検討結果。2026-07-02 の設計議論のまとめ。

- 関連仕様: [mobile-support.md](mobile-support.md)（モバイルネイティブ対応。制約・要求が本メモと多く重なる）
- 関連要望: `LinkSelf拡張要望.md`

---

## 1. 結論サマリ

| 論点 | 結論 |
|------|------|
| Go コアをブラウザで動かす（WASM 化） | **不可**。go-libp2p のトランスポート層が生ソケット前提で、ブラウザに API が存在しない |
| ブラウザ版 LinkSelf | **可能**。js-libp2p 上に LinkSelf プロトコル層を TS で再実装（ワイヤ互換の第二実装） |
| 「サーバー不要」は維持できるか | 部分的。ブラウザピアには**常時稼働ノード**（リレー/ブートストラップ/メールボックス）が必要。ただし信頼を要求する中央サーバーではなく「よく起きているただのピア」 |
| 常時稼働ノードを Vercel 無料枠に | **不可**（サーバーレスは常駐プロセス・任意ポート listen・WebSocket サーバー不可） |
| 常時稼働ノードを Windows ローカルアプリに | **条件付き可**。アプリ形態は問題なし。関門は回線の着信可否（ポート開放、MAP-E/DS-Lite）と稼働率 |
| モバイルネイティブ（gomobile）をノードに | **一般ピアとしては可**（PWA より大幅に高機能）。常時稼働 3 役は**不可**（キャリア CGN で着信不能） |

## 2. ブラウザ版の実現方式

### 2.1 Go WASM ルートが塞がっている理由

`core/go.mod` のスタック（go-libp2p / kad-dht / quic-go / pion WebRTC / mDNS / TCP・UDP）は生ソケット前提。`GOOS=js` でコンパイルしてもブラウザにソケット API がなく、トランスポート層が丸ごと動かない。gomobile と違い回避策なし。

### 2.2 TS 再実装ルートの部品対応

| Go 側部品 | ブラウザ側対応物 | 備考 |
|-----------|------------------|------|
| go-libp2p / kad-dht / Noise / yamux | **js-libp2p**（公式 TS 実装） | ワイヤ互換。DHT はクライアントモードのみ |
| ncruces/go-sqlite3（WASM ベース） | 公式 **sqlite-wasm + OPFS** | 設計思想はほぼ同じ |
| DID (did:key) / 鍵 | WebCrypto | Ed25519 は主要ブラウザ対応済み |
| `core/internal` の LinkSelf 固有プロトコル層 | **TS で再実装（工数の本体）** | auth / envelope / storeforward / syncdb / group / role / permission / devicesync / sqlproxy 等 約 17 パッケージ |

libp2p のプロトコル ID とメッセージ形式を揃えれば Go ノードと相互運用可能。「移植」ではなく「同一プロトコルの第二実装」。

### 2.3 ブラウザ固有の制約（mobile-support.md §2 と同種、より厳しい）

1. **トランスポート**: TCP/QUIC 不可。WebSocket / WebTransport / WebRTC のみ。**Go 側デーモンに `/ws` または `/webtransport` の listen アドレス追加が必要**（go-libp2p は対応済み、Config 追加程度）
2. **着信不可**: ブラウザノードは外から接続を受けられない → **Circuit Relay v2 必須**
3. **DHT**: クライアントモードのみ。ブートストラップノードがブラウザ向けトランスポートを話す必要あり
4. **mDNS / Bluetooth 発見不可**
5. **バックグラウンド実行ゼロ**: タブを閉じれば完全停止（iOS ネイティブの「30 秒猶予」すらない）。FastStart / peerstore 永続化（mobile-support.md §3.1.2〜3.1.4）がブラウザ版でもそのまま必要
6. **通知**: Web Push はプッシュサーバー経由が仕様上必須（mobile-support.md §3.3.1 の APNs/FCM 問題と同型）
7. **TLS 証明書は不要にできる**: libp2p の WebTransport / WebRTC-direct は multiaddr に証明書ハッシュ（certhash）を埋め込む方式のため、**ドメイン取得も CA 証明書もなしで** https 配信の PWA から接続可能。自宅ノードでも詰まない

## 3. 通信トポロジ

### パターン 1: リレー経由（フォールバック）

```
ブラウザA ──WebSocket──▶ リレーノード ◀──WebSocket── ブラウザB
                （Circuit Relay v2 が両接続を突き合わせて転送）
```

双方が常時稼働ノードに WebSocket/WebTransport を張りっぱなしにし、リレーがバイト列を転送。中身は Noise で E2E 暗号化されており**リレーは内容を読めない**（ただの土管。サーバー間連携は存在しない）。

### パターン 2: WebRTC ブラウザ間直結（本命）

```
ブラウザA ──リレー経由でシグナリングのみ──▶ ブラウザB
      ↓ 成立後
ブラウザA ◀════ WebRTC DataChannel（直接） ════▶ ブラウザB
```

リレーは最初の握手（SDP 交換）にのみ使用。定常状態はサーバー非経由。NAT の相性で直結できない場合のみパターン 1 に留まる。

## 4. 常時稼働ノードの 3 役

1. **ブートストラップ** — DHT への入口（最初に multiaddr を知る手段）
2. **リレー** — Circuit Relay v2 による中継・シグナリング
3. **メールボックス（store-and-forward）** — ブラウザピアはタブを閉じると消えるため、宛先オフライン時の暗号化済み envelope を預かる

アカウントも平文データも持たない「よく起きているただのピア」。LinkSelf デスクトップデーモンがそのまま担える。誰が運用しても・複数あっても成立する。「サーバー不要」は「**信頼を要求する中央サーバーが不要**」と読み替える。

## 5. 常時稼働ノードのホスティング検討

### 5.1 Vercel（無料枠含む）— 不可

| 必要なもの | Vercel の実態 |
|-----------|---------------|
| 常駐プロセス | Functions はリクエスト駆動、処理後に凍結・破棄 |
| 受け側 WebSocket 常時保持 | WebSocket サーバーになれない・実行時間上限あり |
| TCP/UDP 任意ポート listen | HTTP(S) 以外不可 |
| メールボックス用永続ディスク | ファイルシステム揮発 |

Vercel に置けるのは: PWA 静的配信、Web Push 送信 API（イベント駆動）、delegated routing 的な HTTP API 程度。

### 5.2 代替ホスティング

1. **Oracle Cloud Always Free**（推奨）: ARM 4コア/24GB RAM VM が恒久無料。フル VM で TCP/UDP/WS 自由
2. **Google Cloud e2-micro**: 恒久無料（米国リージョン、スペック小）。リレー用途なら足りる
3. **自宅デスクトップ/ラズパイ**: 追加コストゼロだが公開到達性の確保が必要（§5.3）
4. **格安 VPS（月数百円）**: 無料にこだわらないなら最も素直

### 5.3 Windows ローカルノード — 条件付き可

アプリ形態は問題なし（Go デーモンをトレイ常駐/Windows サービス化）。関門は 2 点:

**外部到達性（最大の関門）**
- ルーターのポート開放が必要（go-libp2p の UPnP 自動開放は確実ではない）
- **日本の回線特有の罠**: IPoE 系（v6プラス等の MAP-E）は開放可能な IPv4 ポートが共有制で限定、**DS-Lite は IPv4 ポート開放が不可能**。モバイル回線・一部光回線の CGN も同様。**回線契約次第で着信不可が確定する**ため、契約種別（PPPoE か MAP-E/DS-Lite か）の確認が最初のチェック項目
- ブートストラップ役は「固定座標」が必要 → 動的 IP なら DDNS 等
- TLS 証明書は certhash 方式で不要（§2.3-7）

**稼働率**
- スリープ/休止設定、Windows Update 自動再起動、停電後の自動起動（サービス登録）
- 1 台構成ではその 1 台の稼働率＝システム全体の稼働率（落ちている間はメールボックスも入口も止まる）

### 5.4 モバイルネイティブ（gomobile）— 一般ピアのみ可

**一般ピアとしては PWA より大幅に有利**:

| 能力 | PWA（ブラウザ） | ネイティブ |
|------|----------------|-----------|
| TCP / QUIC 直接続 | 不可 | 可 |
| DHT 参加 | クライアントのみ | フル参加可 |
| mDNS（同一 Wi-Fi 発見） | 不可 | 可 |
| Bluetooth 発見 | 実質不可 | 可（Berty 実績） |
| バックグラウンド | ゼロ | Android: Foreground Service で継続可 / iOS: 約 30 秒 |

Android は FGS + 電池最適化除外で画面を閉じても同期継続可（Syncthing / Briar Android 版と同手法）。Go コア流用で TS 再実装も不要。

**常時稼働 3 役は不可**（OS ではなく回線の問題）:
1. **キャリア CGN**: モバイル回線は例外なく CGN 下でポート開放という概念自体がない。着信不能な端末はリレーにもブートストラップにもなれない（アプリ側で回避不能）
2. **iOS**: 背面 30 秒 suspend（Briar が iOS 断念した理由）。常駐は審査的にも技術的にも非現実的
3. **Android**: FGS 常駐は可能でも OEM タスクキラー・電池・データ消費で中継役に不向き

例外は「自宅 Wi-Fi の給電しっぱなし余り Android 端末」（実質ミニサーバー）だが、直面する問題は §5.3 と同一かつ OEM キラー分だけ PC より不安定で、あえて選ぶ理由は薄い。

## 6. 推奨アーキテクチャ

- **ブラウザ版**: js-libp2p + sqlite-wasm で「ブラウザ = リーフノード」設計。純ブラウザ間 WebRTC は後から追加可能
- **モバイル**: gomobile ネイティブで「高機能な一般ピア」に徹する（mobile-support.md 通り）
- **常時稼働 3 役**: デスクトップ/クラウドに固定。**設計上「常時稼働ノードは複数登録できる」形にする**（`BootstrapPeers` / `CircuitRelays` はリスト形式）。まず手元の Windows 機で開始 → 着信不可や稼働率が問題になったら Oracle Cloud Always Free 等のノードを追加、という段階移行がアプリ変更なしでできる。1 台前提で作ると後から効く
- **Vercel**: PWA 配信専用
- Go 側の拡張工事（`/ws`・`/webtransport` listener、Circuit Relay v2、FastStart、peerstore 永続化）は**モバイル対応とブラウザ対応の両方に一度で効く**

## 7. 実装状況（2026-07-07）

home-visit-suite の全面 PWA 化決定（2026-07-07。単一 PWA + ロール別 UI、モバイルネイティブ案は廃止）を受け、Go 側の拡張を実装済み:

| 項目 | 状態 | 内容 |
|------|------|------|
| WebSocket listener | ✅ 実装済み（テストで固定化） | go-libp2p のデフォルトトランスポートに含まれるため、`ListenAddrs` に `/ip4/.../tcp/<port>/ws` を指定するだけで動作。`test/integration/browser_transport_test.go` |
| Circuit Relay v2（サーバー役） | ✅ 実装済み | `Config.EnableRelayService`。**リレーサービスは自ノードが public reachable と判定されるまで起動しない**ため、公開アドレス確定済みノードでは `ForceReachability: "public"` を併用 |
| Circuit Relay v2（クライアント役） | ✅ 実装済み | `Config.CircuitRelays []string`（複数登録可）。static relay へ予約し `/p2p-circuit` アドレスを広告。着信不能が確定しているリーフは `ForceReachability: "private"` で即予約 |
| リレー経由ストリーム | ✅ 実装済み | relayed conn は libp2p 上 limited 扱いのため、auth / メッセージのストリーム開設で `network.WithAllowLimitedConn` を許可（`internal/node`） |
| デーモン露出 | ✅ 実装済み | `linkself-daemon` の `start` パラメータに `circuitRelays` / `enableRelayService` / `forceReachability` を追加 |
| WebTransport listener | 未検証 | デフォルトトランスポートに含まれる（`/udp/<port>/quic-v1/webtransport`）。js-libp2p との疎通 PoC（M1）で検証する |
| FastStart / peerstore 永続化 | ❌ 未実装 | mobile-support.md §3.1.3 / §3.1.4 参照 |
| メールボックス（store-and-forward） | ⚠️ 既存実装あり | `internal/storeforward`。常時稼働ノードでの預かり運用（ディスク永続化・容量管理）は未整備 |
| M1 疎通 PoC（js-libp2p ↔ Go） | ✅ **PASSED**（2026-07-07） | `ts/poc-m1/`。WebSocket 直結 + Circuit Relay v2 経由の両経路で `/linkself/msg/1.0.0` の双方向往復に成功。Noise / yamux / uint32 framing / did:key 導出の互換を確認。js 側は `runOnLimitedConnection: true` が必要（Go 側 `WithAllowLimitedConn` と対） |
| TS 実装（プロトコル層） | ✅ **M2 完了**（2026-07-07） | `ts/linkself/`（`@linkself/core`）。did / framing / envelope / auth / node / syncdb / group / role / permission / groupshare / devicesync / sqlproxy / network / pairing / **LinkSelfClient 組み立て**。123 テスト GREEN、ワイヤ形式は Go golden 一致、auth/node は Go 相手 live interop、Client は TS 2 ノード e2e 済み（詳細は `ts/linkself/README.md`） |
| Go↔TS groupshare interop | ✅ **双方向 PASSED**（2026-07-07） | `core/cmd/poc-gsnode` + `ts/linkself/test/groupshare.interop.test.ts`。SharedRecord の Go→TS / TS→Go 双方向適用を実ワイヤで確認（Go 公開 API は groupshare 未露出のため internal 直結ハーネス。Phase C 後に pkg/linkself ベースへ置換） |
| M3: sqlite-wasm + MyDB | ⚠️ **コア完了**（2026-07-07） | `SqliteWasmDatabase`（ブラウザで OPFS VFS 自動選択）+ `MyDB`（KV+SQL、SQL 書き込みの devicesync ミラー = Go と同一配線）。残: ブラウザ実機確認（COOP/COEP・Web Locks 多タブ直列化）・dataroot 相当 OPFS レイアウト・FastStart/peerstore 永続化 |

実装ノート:
- **autorelay は public でないリレーアドレスを広告対象から除外する**（`cleanupAddressSet`）。loopback 上のテストや LAN 内リレーでは `/p2p-circuit` アドレスが `Host.Addrs()` に現れないため、テストでは circuit アドレスを手動構築して接続確認している。公開アドレスを持つ本番リレーでは自動的に広告される

## 8. 本メモの位置づけ

設計議論の記録（検討メモ）＋ Go 側実装状況の追跡。実装スコープ・優先度が確定したら mobile-support.md 同様の正式仕様に昇格させるか、該当仕様へ参照を移す。home-visit-suite 側の要望原稿は同リポジトリ `docs/wants/11_LinkSelf拡張要望.md`（PWA 版に改訂済み）を参照。
