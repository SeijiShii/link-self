# 実装変更履歴（要約）

このドキュメントは、ここまでの実装で廃止・変更した内容をまとめたものです。

---

## 1. DHT モード

- **廃止**: `UsePublicDHT` 設定（false = LinkSelf 専用 DHT、true = 公開 DHT の切り替え）
- **現状**: **常に公開 DHT**（`ProtocolPrefix("/ipfs")`）のみを使用。FindPeer(DIDToPeerID(did)) で相手を検索。LinkSelf 専用 DHT（/linkself, PutDID/FindDID）は使用しない。

---

## 2. 接続方法

- **廃止**: 相手の Listen アドレスを入力しての「直接接続」（DHT 検索をスキップする接続）
- **現状**: 接続は **DHT のみ**。友達申請時は **DID のみ**を入力。クライアントからは `connect(peerDID)` のみ呼び、DHT で相手を検索して接続する。

---

## 3. チャットクライアントの Listen 表示

- **廃止**: UI での「Listen: ...」表示および「DID + Listen（結合形式）をコピー」機能
- **現状**: **DID のみ**表示・コピー。自分のリッスンアドレスは UI に表示しない（daemon は Start 結果で返すがクライアントでは未使用）。

---

## 4. 関連する API・型

| 対象 | 廃止・変更内容 |
|------|----------------|
| core `Config` / StartParams | `UsePublicDHT` フィールド削除 |
| core `Connect` | クライアントからは `peerDID` のみ渡す（listenAddr は未使用） |
| chat-client `ConnectParams` | `listenAddr` 削除 |
| chat-client `useLinkSelf` | `listenAddr` state 削除、`connect(peerDID)` のみ |
| chat-client 友達申請 | 貼り付けは DID のみ抽出・使用 |

---

## 5. ドキュメントの参照

- テスト動作: [entry-point.md](entry-point.md)
- core DHT 仕様: [core/README.md](../../core/README.md)、[core/pkg/linkself/README.md](../../core/pkg/linkself/README.md)
