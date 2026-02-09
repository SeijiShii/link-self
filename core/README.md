# LinkSelf Core

基本モジュール（Go）。PC / Android / iOS で共有するコア。Phase 1 で実装した DID・DHT・認証・Store-and-Forward の定義とパッケージ構成。

**グループ**: メッセージ・接続は**グループ単位**を想定している。グループ = メンバー DID の集合（2 人以上）。1 対 1 は 2 人グループのケース。詳細は [docs/group-concept.md](../docs/group-concept.md) を参照。送信・接続 API は SendToGroup / ConnectToGroup への移行を計画している（現状実装は SendMessage / Connect のまま）。

## パッケージ構成

```
core/
├── go.mod                 # module github.com/SeijiShii/link-self/core
├── internal/
│   ├── did/               # did:key 生成・パース・検証（Ed25519）
│   ├── dht/               # DHT の Provide/Find のラップ（DID ↔ AddrInfo）
│   ├── auth/              # チャレンジ・レスポンス認証
│   ├── node/              # ノード = Host + DHT + Auth + Store-and-Forward
│   └── storeforward/      # メッセージキューとオンライン検知時の送信
├── pkg/
│   └── linkself/          # 公開API（疎結合化のため作成）
│       ├── types.go       # Clientインターフェースと型定義
│       ├── client.go      # Client実装
│       └── README.md      # APIドキュメント
└── cmd/
    └── linkself-daemon/   # JSON-RPC daemon（Electronアプリ用）
```

## 公開API

`pkg/linkself`パッケージは、LinkSelfコアライブラリの公開APIを提供します。

- **ドキュメント**: [pkg/linkself/README.md](pkg/linkself/README.md)
- **オンライン**: https://pkg.go.dev/github.com/SeijiShii/link-self/core/pkg/linkself（公開後）
- **ローカル**: `go doc github.com/SeijiShii/link-self/core/pkg/linkself`

## DID ↔ DHT キー・メッセージ形式

- **DID**: `did:key:` + multibase(base58btc, multicodec(0xed) + raw Ed25519 公開鍵 32 バイト)。
- **DHT キー**: `/linkself/did/` + base32(SHA256(DID 文字列))。DHT は `ProtocolPrefix("/linkself")` で他 DHT と分離。
- **DHT 値**: JSON 化した `peer.AddrInfo`（ID + Addrs）。
- **メッセージプロトコル**: `/linkself/msg/1.0.0`。ペイロードは 4 バイト BigEndian 長 + 本体。

## ビルド・テスト

```bash
cd core
go build ./...
go test ./internal/... ./test/...
```

## ローカル多ノードテスト（5 シナリオ）

`go test ./test/integration/...` で以下を網羅:

1. **DID 生成・一致**: 同一秘密鍵から同じ DID が得られること。
2. **DHT 登録・発見**: Node A が Provide → Node B が Find で A のアドレスを取得できること。
3. **接続・認証**: B が A の DID で接続し、チャレンジ・レスポンスで認証成功すること。
4. **メッセージ（オンライン）**: A と B がともにオンラインのとき、B→A のメッセージが A に届くこと（2 人グループのケース）。
5. **メッセージ（Store-and-Forward）**: B が A にメッセージを送るが A はオフライン → B がキューに保持 → A が起動して B が Connect でフラッシュ → A が受信すること。

注: グループ概念・オーナー・脱退などは [docs/group-concept.md](../docs/group-concept.md) を参照。他計画は [docs/phase1-design.md](../docs/phase1-design.md)、[docs/sync-db-plan.md](../docs/sync-db-plan.md)、[docs/sample-chat-app-plan.md](../docs/sample-chat-app-plan.md) を参照。
