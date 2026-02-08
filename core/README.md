# LinkSelf Core

基本モジュール（Go）。PC / Android / iOS で共有するコア。Phase 1 で実装した DID・DHT・認証・Store-and-Forward の定義とパッケージ構成。

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
└── cmd/linkself/
```

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
4. **メッセージ（オンライン）**: A と B がともにオンラインのとき、B→A のメッセージが A に届くこと。
5. **メッセージ（Store-and-Forward）**: B が A にメッセージを送るが A はオフライン → B がキューに保持 → A が起動して B が Connect でフラッシュ → A が受信すること。
