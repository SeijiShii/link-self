# テスト動作のエントリポイント

**テスト動作させたいとき**は、このドキュメントを起点に手順を進めてください。

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

**実運用を想定した検証について**: 現状の公開DHT（bootstrap.libp2p.io）は **LinkSelf のキー（/linkself/did/）を共有しない**可能性があり、その場合 2 インスタンス同士で connect が失敗します。**IPFS形式（公開DHT）**で接続を試す場合は [「IPFS形式（USE_PUBLIC_DHT）で接続する」](#ipfs形式use_public_dhtで接続する) を参照。**実運用と同等の検証**をするには、[「3. 実運用を想定した検証」](#3-実運用を想定した検証) を参照し、LinkSelf 用 bootstrap ノードの利用または BOOTSTRAP_PEER による検証を行ってください。

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

   **connect が失敗する場合**: 公開DHT（bootstrap.libp2p.io）は LinkSelf の DID 用キーを共有しない可能性があります。**実運用を想定した検証**では、userA の Listen アドレスをコピーし、userB を **BOOTSTRAP_PEER** で起動して「同じ DHT」に参加させてください。  
   ```bash
   # userA のヘッダー「Listen:」横のコピーボタンでアドレスをコピーしてから
   BOOTSTRAP_PEER='/ip4/127.0.0.1/tcp/xxxxx/p2p/...' npm run electron:userB
   ```  
   公開DHTを使わない場合は `DISABLE_PUBLIC_DHT=1 npm run electron:userB` で無効化できます。

### 確認したい流れ

1. **A の DID を B に渡す** — A のヘッダーに表示されている DID をコピー。
2. **B が友達申請を送る** — B の連絡先ヘッダー「＋」→ DID を貼り付け →「友達申請を送る」。
3. **A が申請を承認** — A の「友達申請」一覧に B が表示される →「承認」。
4. **メッセージ送受信** — A または B で相手を選択し、メッセージを送信。もう一方で受信されることを確認。

仕様・フロー詳細は [友達追加・申請承認・複数インスタンス](friend-add-and-multi-instance.md) を参照。

---

## 3. 実運用を想定した検証

実運用では、**ユーザー同士が同じ DHT 上で DID を検索し、つながれる**必要があります。そのためには **LinkSelf 用の DHT 名前空間（/linkself/did/）を扱う bootstrap ノード**が同じネットワーク上にいる必要があります。

### 現状

- **公開DHT（bootstrap.libp2p.io）**: IPFS/libp2p 用の名前空間を前提としており、**LinkSelf の /linkself/did/ キーを保存・返却しない**可能性が高い。そのため、公開DHTだけでは connect が失敗することがある。
- **ローカル検証**: userB を **BOOTSTRAP_PEER=userA の Listen アドレス** で起動すると、userA と userB が同じ小さな DHT を形成し、互いの DID を検索できる。実装・手順は「2. 2 者間でテスト」の補足のとおり。

### 公開DHTに働きかけるには

LinkSelf は DHT のプロトコルプレフィックスに **`/linkself`** を使い、キーは **`/linkself/did/`** 名前空間で PutDID / FindDID しています。公開 bootstrap（bootstrap.libp2p.io）は **別のプロトコル**（IPFS 用 DHT）で動いているため、/linkself のキーを保存・返却せず、「同じ DHT」に参加していません。

**取りうる方法は次の 2 通りです。**

1. **公開DHTのプロトコルに合わせる**  
   core の DHT を「公開 bootstrap が話すプロトコル」（例: IPFS 用のデフォルト）で動かすように変更し、キーをその名前空間で保存する。  
   - **難点**: 公開 bootstrap ノードが任意のキー（/linkself/did/ 相当）を受け入れるか、LinkSelf 用の Validator を持っているかは不明。多くの公開 DHT は決まったスキーマだけ受け付けるため、**現実的には難しい**可能性が高い。  
   - 詳細は下記「[公開DHTを経由する方法（詳細）](#公開dhtを経由する方法詳細)」を参照。

2. **LinkSelf 用 bootstrap ノードを自分で立てる（推奨）**  
   LinkSelf と同じ設定（`ProtocolPrefix("/linkself")`、LinkselfValidator）で動くノードを 1 台以上用意し、その **Listen アドレス**を全クライアントの BOOTSTRAP_PEER（またはデフォルト bootstrap リスト）に設定する。  
   - 全員がその bootstrap に接続することで「LinkSelf 用の DHT」が形成され、PutDID / FindDID が機能する。  
   - そのノードは **LinkSelf を実装したアプリを 1 台、安定したアドレスで起動したもの**でよい（専用バイナリは必須ではない）。  
   - これが「公開DHTに働きかける」の代わりに **LinkSelf 専用の DHT を用意する**現実的な方法です。

### 公開DHTを経由する方法（詳細）

公開DHT（bootstrap.libp2p.io など）を経由して DID を検索するには、**同じ DHT ネットワークに参加し、かつ公開ノードが受け付けるキー・値の形式で保存する**必要があります。

#### なぜ今は繋がらないか（技術的な理由）

| 項目 | LinkSelf（現状） | 公開DHT（bootstrap.libp2p.io） |
|------|------------------|--------------------------------|
| **DHT プロトコル** | `ProtocolPrefix("/linkself")`（core/internal/node/node.go） | デフォルトは **`/ipfs`**（IPFS Kademlia DHT） |
| **参加するネットワーク** | 「/linkself を話すノード」だけが同じルーティングテーブルを共有 | 「/ipfs を話すノード」が同じルーティングテーブルを共有 |
| **キー形式** | `/linkself/did/` + base32(sha256(did))（core/internal/dht/dht.go） | レコードは **名前空間** で区切られる（例: `/pk/`, `/ipns/`） |
| **値の形式** | JSON の peer.AddrInfo | 名前空間ごとに **Validator** で検証（形式が違うと拒否される） |
| **デフォルトの Validator** | LinkSelf 用: `/linkself/did/` のみ受け付ける | **`pk`**（公開鍵→PeerID）、**`ipns`**（IPNS レコード）のみ |

- LinkSelf ノードは **/linkself** プロトコルで DHT に参加しているため、**/ipfs** で動く公開 bootstrap ノードとは **別の DHT** になっている（ルーティングテーブルを共有しない）。
- 仮に LinkSelf 側を **/ipfs** に合わせても、PutValue するキーは `/linkself/did/...` の形式。公開側のノードは **pk / ipns 以外の名前空間** をどう扱うかは実装依存で、**知らない名前空間を拒否する**可能性が高い。そのため、`/linkself/did/` のキーが保存・返却される保証はない。

#### 公開DHT経由で動かすには（必要な変更）

1. **同じプロトコルで参加する**  
   core の DHT 作成時に `ProtocolPrefix("/ipfs")`（または公開 bootstrap が実際に使っている ID）に変更する。これで「同じ DHT ネットワーク」に参加できる。

2. **保存するキー・値を公開側が受け付ける形式にする**  
   公開 DHT のデフォルト Validator が受け付けるのは **pk** と **ipns** 名前空間のみ。  
   - **pk**: 公開鍵 → PeerID のマッピング。値は PeerID など決まった形式。LinkSelf がやりたい「DID → peer.AddrInfo（複数アドレス）」をそのまま入れる形式ではない。  
   - **ipns**: IPNS レコード用。IPNS の仕様に沿った形式が必要。LinkSelf の AddrInfo を IPNS レコードに載せる拡張は仕様外で、互換性や他ノードの扱いが不明。  
   - **独自名前空間（linkself）**: 公開 bootstrap ノードのソフトウェアは **pk / ipns のみ** を想定しているため、`/linkself/did/` をそのまま使うと **Validator が存在せず拒否される**可能性が高い。libp2p 本体に「linkself」名前空間を追加する変更が入らない限り、公開ノード側で受け入れられる見込みは薄い。

3. **まとめ（公開DHT経由の現実性）**  
   - **理論上**: ProtocolPrefix を /ipfs に合わせ、さらに「pk または ipns の形式に DID→AddrInfo をエンコードする」方式を設計すれば、公開DHT経由で DID 検索ができる可能性はある。  
   - **現実には**: (a) pk/ipns の既存形式に無理に載せるのは仕様と実装の両方で複雑、(b) 公開 bootstrap の実装・運用は LinkSelf が変えられない、(c) 将来 libp2p が linkself 名前空間を標準でサポートすれば話は変わるが、それは LinkSelf 単体ではコントロールできない。  
   - そのため、**現時点では「公開DHTを経由する」より「LinkSelf 用 bootstrap を 1 台立てる」方が現実的**です。

#### 公開DHTのプロトコルに合わせる場合の問題

LinkSelf を公開DHT（/ipfs）に合わせるうえで、具体的に起きる問題は次のとおりです。

| 問題 | 内容 |
|------|------|
| **1. 未知の名前空間は拒否される** | libp2p の DHT では、PutValue で受け取ったキーの**名前空間ごとに Validator が必須**。Validator が登録されていない名前空間のレコードは**拒否される**。公開 bootstrap ノードは **pk / ipns のみ** Validator を持っているため、**/linkself/did/ のキーは保存されない**（レプリケーション先で拒否される）。 |
| **2. 既存の名前空間（pk / ipns）に載せる難しさ** | 受け入れてもらうには、キー・値を **pk または ipns の形式**に合わせる必要がある。**pk**: 公開鍵→PeerID などの決まったスキーマ。LinkSelf が欲しいのは「DID → peer.AddrInfo（複数アドレス）」であり、形式が合わず**流用しづらい**。**ipns**: IPNS レコードは署名・シーケンス・TTL など仕様が決まっており、AddrInfo をそのまま載せるのは**仕様外**。無理にエンコードすると他ノードやツールの挙動が読めない。 |
| **3. 公開ノードの挙動は変えられない** | bootstrap.libp2p.io など**公開側のノードのソフトウェア・Validator は LinkSelf が変更できない**。LinkSelf は「クライアント側の変更」だけ可能で、**「linkself 名前空間を標準で受け入れる」ようにするには libp2p 本体の変更**が必要。 |
| **4. 実装・検証の手間** | 仮に pk/ipns のどちらかに「DID→AddrInfo」を載せる方式を設計しても、(a) 仕様策定・エンコード形式の設計、(b) core の DHT クライアントの変更（ProtocolPrefix、キー・値の生成）、(c) 実際の公開DHTへの Put/Get での動作確認、が必要。公開DHTのノードバージョン差で**挙動が変わるリスク**もある。 |

**結論**: プロトコルを /ipfs に合わせる**コード上の変更は 1 箇所で済む**が、「/linkself/did/ のまま」では公開ノードに**保存してもらえない**。既存の名前空間に載せるには**仕様・実装の両方で大きな作業**があり、かつ公開側の仕様に依存するため、**難易度は高い**。

### IPFS形式（USE_PUBLIC_DHT）で接続する

core を **公開DHTのプロトコル（/ipfs）に合わせる**モードで起動すると、PutDID/FindDID の代わりに **FindPeer(DIDToPeerID)** で相手を探します。DID から PeerID を導出し、公開 DHT の Peer ルーティングで検索するため、**LinkSelf 用 bootstrap ノードなし**で DID による connect が動く可能性があります。

**手順**

1. **両方のインスタンス**を `USE_PUBLIC_DHT=1` で起動する。  
   ```bash
   # ターミナル 1
   USE_PUBLIC_DHT=1 npm run dev:userA
   # ターミナル 2（Vite 起動後）
   USE_PUBLIC_DHT=1 npm run electron:userB
   ```
2. デフォルトの bootstrap（bootstrap.libp2p.io）が使われ、両者が同じ公開 DHT に参加する。
3. 友達申請で相手の DID を入力すると、**FindPeer(PeerID)** で相手を探し、見つかれば接続する。

**注意**

- 公開 DHT の FindPeer は「ルーティングテーブルにいるピア」を探すため、**両者が公開 DHT に接続され、かつ相互に到達可能**である必要がある。NAT やファイアウォールで直接つながらない場合は失敗することがある。
- 接続に失敗する場合は、従来どおり **BOOTSTRAP_PEER** で LinkSelf 用 bootstrap（または userA の Listen）を指定する方法を使う。

### 実運用に近い検証をするには

1. **LinkSelf 用 bootstrap ノードを運用する**  
   core の DHT（/linkself/did/ を扱う）を起動したノードを 1 台以上用意し、その **Listen アドレス** をアプリのデフォルト bootstrap に設定する。検証時も実運用時も、全ユーザーがその bootstrap に接続することで「同じ LinkSelf DHT」に参加し、DID で互いを発見できる。  
   **補足**: 専用の「bootstrap 用バイナリ」は必須ではない。**LinkSelf を実装したアプリ**（同じ core を使うチャットアプリなど）を 1 台、安定したアドレスで起動し、その Listen アドレスをデフォルト bootstrap に設定すれば、そのアプリが「LinkSelf 用 bootstrap ノード」になる。

2. **検証手順（BOOTSTRAP_PEER で再現）**  
   LinkSelf bootstrap ノードをまだ用意していない場合は、**userA を「1 台目の bootstrap」とみなして**、userB を `BOOTSTRAP_PEER=userAのListenアドレス` で起動する。これで「実運用で 1 台の bootstrap に全員がつながる」状態をローカルで再現できる。

3. **今後のタスク**  
   - LinkSelf 用 bootstrap の用意：同一アプリ（または core を使う daemon）を 1 台、安定したアドレスで起動し、その Listen をデフォルト bootstrap に設定する。  
   - アプリのデフォルト bootstrap を、そのノードのアドレスに切り替える。  
   - 上記が整ったうえで、BOOTSTRAP_PEER なしで 2 インスタンス同士が connect できることを確認する。

---

## 4. ドキュメント一覧（テスト・開発時に参照するもの）

| ドキュメント | 内容 |
|-------------|------|
| **[entry-point.md](entry-point.md)**（本ファイル） | テスト動作の入口。最小手順と 2 者間テストの起動方法。 |
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
| daemon のビルド | `npm run build:daemon` |
| アプリのビルド | `npm run build` |
