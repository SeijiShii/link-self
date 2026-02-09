# テスト動作のエントリポイント

**テスト動作させたいとき**は、このドキュメントを起点に手順を進めてください。

**補足**: LinkSelf daemon のソースは **core 内（core/cmd/linkself-daemon）** にのみあります。chat-client 内に daemon ディレクトリはありません。`npm run build:daemon` は core の daemon をビルドします。

**現在の仕様**: DHT は **常に公開 DHT**（/ipfs）を使用。接続は **DID のみ**で行い、DHT で相手を検索（FindPeer）して接続する。UI では **DID のみ**表示・コピー（Listen は表示しない）。詳細は [implementation-changes.md](implementation-changes.md) を参照。

---

## 1. 最小手順（1 インスタンスで起動）

```bash
cd chat-client
npm install
npm run build:daemon
npm run dev
```

- 起動後、ヘッダーに「My DID: ...」が表示されれば daemon と UI の接続は成功です。
- 詳細な前提条件・トラブルシュートは [テスト動作ガイド](testing-guide.md) を参照。

---

## 2. 2 者間でテストしたいとき（複数インスタンス）

2 つのウィンドウで「ユーザー A」と「ユーザー B」を再現し、友達申請・承認・メッセージ送受信を確認する手順です。

**重要**: 2 つのインスタンスで**別の DID**にするには、必ず **1 つ目は `dev:userA`、2 つ目は `electron:userB`** を使ってください。両方とも `npm run dev` で起動すると、同じ identity を使うため**同じ DID**になります。ヘッダーに表示される「userA」「userB」「default」で、どのプロファイルで動いているか確認できます。

### 手順

1. **ターミナル 1**（ユーザー A 用） — **必ず先に起動する**
   ```bash
   cd chat-client
   npm run dev:userA
   ```
   → Vite と 1 つ目の Electron が起動。ヘッダーに「userA」と出ていれば OK。このウィンドウは `./data/userA` に identity・連絡先を保存します。**Vite が `http://localhost:5173` で動くため、2 つ目のウィンドウ（userB）はこのあとから起動する。**

2. **ターミナル 2**（ユーザー B 用） — ターミナル 1 で Vite が起動してから実行する
   （Vite が動いていない状態で electron:userB だけ起動すると白画面になります）
   ```bash
   cd chat-client
   npm run electron:userB
   ```
   → 2 つ目の Electron が起動。ヘッダーに「userB」と出ていれば OK。

   **connect が失敗する場合**: デフォルトでは公開 DHT（bootstrap.libp2p.io）に参加します。同一マシンや NAT 内で 2 インスタンスだけの場合は、公開 DHT 上で互いに見つからないことがあります。その場合は、userA のノードを bootstrap として userB を起動してください。userA のリッスンアドレスは、userA 側の daemon 起動時のログや、daemon の Start 結果で確認できます（UI には表示しません）。
   ```bash
   # userA のリッスンアドレスを BOOTSTRAP_PEER に指定して userB を起動
   BOOTSTRAP_PEER='/ip4/127.0.0.1/tcp/xxxxx/p2p/...' npm run electron:userB
   ```
   デフォルトの bootstrap を使わない場合は `DISABLE_PUBLIC_DHT=1` を付け、必ず `BOOTSTRAP_PEER` で 1 台以上のアドレスを指定してください。

### 確認したい流れ

1. **A の DID を B に渡す** — A のヘッダーに表示されている DID をコピー（DID のみコピーボタン）。
2. **B が友達申請を送る** — B の連絡先ヘッダー「＋」→ DID を貼り付け →「友達申請を送る」。
3. **A が申請を承認** — A の「友達申請」一覧に B が表示される →「承認」。
4. **メッセージ送受信** — A または B で相手を選択し、メッセージを送信。もう一方で受信されることを確認。

仕様・フロー詳細は [友達追加・申請承認・複数インスタンス](friend-add-and-multi-instance.md) を参照。

---

## 3. DHT と接続の概要

- **DHT**: 常に公開 DHT（`ProtocolPrefix("/ipfs")`）に参加。デフォルトの bootstrap は bootstrap.libp2p.io。BootstrapPeers は起動パラメータで変更可能。
- **接続**: 相手の **DID のみ**を指定。DHT の FindPeer(DIDToPeerID(did)) で相手のアドレスを検索し、接続・認証する。
- **ローカル検証**: 2 台だけで確実に接続したい場合は、先に起動したインスタンスのリッスンアドレスを `BOOTSTRAP_PEER` に指定して 2 台目を起動する。リッスンアドレスは daemon の Start 結果（またはログ）で確認できる。

---

## 4. ドキュメント一覧（テスト・開発時に参照するもの）

| ドキュメント | 内容 |
|-------------|------|
| **[entry-point.md](entry-point.md)**（本ファイル） | テスト動作の入口。最小手順と 2 者間テストの起動方法。 |
| [implementation-changes.md](implementation-changes.md) | 廃止・変更した実装内容の要約。 |
| [testing-guide.md](testing-guide.md) | 前提条件・セットアップ・開発モード・ビルド・トラブルシュートの詳細。 |
| [friend-add-and-multi-instance.md](friend-add-and-multi-instance.md) | 友達追加・申請承認・複数インスタンスの仕様と実装方針。 |
| [implementation-plan.md](implementation-plan.md) | チャットアプリ全体の実装計画・構成。 |

---

## 5. よく使うコマンド

| 目的 | コマンド |
|------|----------|
| 1 インスタンスで起動 | `npm run dev` |
| 2 者間テスト（A 側） | `npm run dev:userA` |
| 2 者間テスト（B 側） | `npm run electron:userB` |
| daemon のビルド | `npm run build:daemon`（core/cmd/linkself-daemon をビルド） |
| アプリのビルド | `npm run build` |
