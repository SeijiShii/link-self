# LinkSelf 永続化と複数アプリ共有の Phase 2 方針

**概要:** LinkSelf ではユーザーが望む実存（DID）を選べる。デバイス上の保存構造は「LinkSelf データ > DID 空間 > アプリごとのデータ」とし、アプリデータはユーザーID（DID）ごとに分離する。チャットクライアントの複数インスタンス起動時も DID を選択して利用する。  
**参照:** [sync-db 計画](sync-db-plan.md)、[サンプルチャットアプリ計画](sample-chat-app-plan.md)、[Phase 1 設計](phase1-design.md)

---

## 1. 現状の永続化方式（参考）

チャットクライアントは **Electron の `app.getPath('userData')`** を基準に、**JSON ファイル** のみで永続化しています。データは**各アプリの userData** にあり、LinkSelf は使っていない。

| データ種別 | 保存先 | 形式 | 備考 |
|------------|--------|------|------|
| identity | `userData/identity.json` | JSON | daemon 起動時に `identityPath` として渡す |
| 連絡先（contacts） | `userData/contacts.json` | JSON 配列 | [main.ts](../chat-client/src/main.ts) の `readContacts` / `writeContacts` |
| 友達申請（friend-requests） | `userData/friend-requests.json` | JSON 配列 | `readFriendRequests` / `writeFriendRequests` |
| **チャットメッセージ** | **永続化なし** | — | React の `useState<Message[]>` のみ。タブ切り替え・再起動で消える |

- ユーザー分離は **`--user-data-dir=./data/userA`** などで `userData` を切り替えて実現（`dev:userA` / `electron:userB`）。
- メイン・プリロード・レンダラー間は IPC（`contacts:get` / `contacts:add` 等）で連携。

**daemon 側:** [core/cmd/linkself-daemon/main.go](../core/cmd/linkself-daemon/main.go) は `linkself.Client` の **SendMessage / Connect / SetOnMessage** のみ使用。**SyncLayer / sync-db は未使用**です。送信は内部で `node.SendToGroup`（2人グループ）で行われています。

---

## 2. 方針：ユーザーは実存を選べる、データは DID ごとに分離

- **LinkSelf ではユーザーは自分の望む実存（DID）を選べる**。複数の実存を保持し、利用時にどれを使うか選択できる。
- **アプリデータ内はユーザーID（DID）ごとに分離**する。アプリ層の仕様によるが、**保存されている実存の一覧から選んでライブラリを利用**できる。
- **チャットクライアントを複数インスタンスで立ち上げるときも、DID を選択**する（例: 起動時や設定で「どの DID で使うか」を選ぶ）。従来の userA/userB のような固定プロファイルではなく、**選択した DID に紐づくデータ**を読む。
- **デバイス内の保存構造**は **LinkSelf データ > DID 空間 > アプリごとのデータ** とする。

---

## 3. 保存構造：LinkSelf データ > DID 空間 > アプリごとのデータ

LinkSelf が管理するデータは、次の **3 階層** で配置する。Windows では AppData などプラットフォーム標準のユーザーデータディレクトリを LinkSelf データルートとする。

| 階層 | 説明 | 例（パス） |
|------|------|-------------|
| **LinkSelf データ** | デバイス上の LinkSelf ルート。環境変数で上書き可。 | `%LOCALAPPDATA%\LinkSelf`（Windows）など |
| **DID 空間** | 実存（DID）ごとのディレクトリ。1 実存＝1 ディレクトリ。 | `LinkSelf/<DID>/`（DID はファイル名安全な表現に変換） |
| **アプリごとのデータ** | その DID でそのアプリが使うデータ。DID 空間の下にアプリIDで分離。アプリID は初期化時にアプリが渡す。 | `LinkSelf/<DID>/apps/<app-id>/`（app-id は例: `com.example.chat-client`） |

**プラットフォーム別 LinkSelf データルート（案）:**

| プラットフォーム | LinkSelf データルート |
|------------------|----------------------|
| **Windows** | `%LOCALAPPDATA%\LinkSelf` または `%APPDATA%\LinkSelf` |
| **macOS** | `~/Library/Application Support/LinkSelf` |
| **Linux** | `~/.local/share/link-self` または `$XDG_DATA_HOME/link-self` |

- **DID 空間の下**に、LinkSelf が管理する **identity（鍵素材）、contacts、friend_requests、groups、sync.db** を置く。同一 DID を選んだアプリは同じ「つながり」を参照する。
- **アプリごとのデータ**は、その DID でそのアプリが使う設定やキャッシュなど。DID 空間の下に **アプリID ごとのフォルダ** で分離する。Android のパッケージ名のように、アプリ間で衝突しない **安全なアプリID** が必要となる。**LinkSelf を利用するアプリ側が、sync-db などにアクセスするとき（ライブラリ／daemon のインスタンスを初期化するとき）にアプリID を渡す**ことに任せる。LinkSelf は中央で ID を発行せず、アプリが一意な ID（例: 逆ドメイン `com.example.chat-client`）を選ぶ責任を持つ。

---

## 4. アーキテクチャ図

```mermaid
flowchart TB
  subgraph device [1 台のデバイス]
    subgraph apps [LinkSelf 利用アプリ]
      AppA[アプリ A]
      AppB[アプリ B]
    end
    subgraph linkself [LinkSelf]
      Daemon[daemon または ライブラリ]
      Daemon --> SelectDID[DID 選択]
      SelectDID --> DID1[DID 空間 1]
      SelectDID --> DID2[DID 空間 2]
    end
    subgraph linkself_data [LinkSelf データ]
      subgraph did_space1 [DID 空間]
        Identity[identity]
        Contacts[contacts / groups]
        SyncDB[sync.db]
      end
      subgraph app_data [アプリごとのデータ]
        AppDataA[chat-client 等]
      end
    end
  end
  AppA -->|"API (利用する DID を指定)"| Daemon
  AppB -->|"API (利用する DID を指定)"| Daemon
  Daemon -->|"読み書き（選択された DID の空間）"| linkself_data
```

- **LinkSelf データ**: ルートの下に **DID 空間**（実存ごと）を並べ、その下に **identity、contacts、groups、sync.db** および **アプリごとのデータ** を配置する。
- **複数アプリ・複数インスタンス**: アプリは利用する **DID を選択**し、その DID に紐づくデータだけを LinkSelf 経由で読み書きする。チャットクライアントを複数起動する場合も、起動時や設定で「DID を選ぶ」ことで、同じ DID を共有したり別 DID を使い分けたりできる。

---

## 5. データの役割分担（更新後）

いずれも **LinkSelf データ > 選択された DID 空間** の下に配置する。

| データ | 保存先（DID 空間内） | 管理者 | 役割 |
|--------|----------------------|--------|------|
| identity（鍵素材） | DID 空間直下（例: identity.json） | LinkSelf | その実存の識別子。daemon は「利用する DID」に応じてここから読む。 |
| contacts, friend_requests | 同上（例: user.db または JSON） | LinkSelf | 友だち登録・申請。その DID に紐づく。 |
| groups（グループ情報・メンバーシップ） | 同上 | LinkSelf | グループ定義。その DID に紐づく。 |
| sync-db（アプリ定義テーブル） | DID 空間内（アプリID で名前空間分け）。実体は RecordStorage。 | アプリが DB クライアント経由でテーブル定義・CRUD。同期は LinkSelf が担当。 | アプリがテーブルを定義し SQLite 的な操作を実行。後勝ちで他端末と同期。 |
| アプリごとのデータ | DID 空間内 `apps/<app-id>/`（app-id は初期化時にアプリが渡す） | アプリ層 | その DID でそのアプリが使う設定・キャッシュ等。 |

- **チャットクライアント**は、contacts / friend_requests / groups を自前の userData には持たず、**利用する DID とアプリID を指定して** LinkSelf の API（daemon の RPC）を呼ぶ。daemon は指定された DID の空間と、その下の `apps/<app-id>/` だけを読み書きする。
- 複数インスタンス起動時は、**起動引数や UI で DID を選択**し、選んだ DID に紐づくデータでライブラリを利用する。

### 5.5 データ同期の二層構造（DeviceSync / GroupShare）

> **注意（2026-03）:** 旧 sync-db（単一 SyncLayer）は **DeviceSync / GroupShare 二層アーキテクチャ** に移行した。詳細は [sync-db-plan.md](sync-db-plan.md) を参照。

- **DeviceSync（同一 DID 間）**: DID 空間内の全データ（contacts、messages、アプリデータ等）を同一 DID の全デバイス間で**透過的に全同期**する。アプリは同期を意識しない。`DeviceDB.Put("contacts", id, body)` のようにローカル DB として使うと、自動的に他デバイスに複製される。
- **GroupShare（異なる DID 間）**: アプリが **Channel**（名前・スキーマ・権限）を定義し、**アプリが選んだ共有データのみ**をグループメンバーに送る。サーバーサイド API のように振る舞う。`AccessPolicy` / `SchemaValidator` はアプリが実装する。
- **ストレージ**: 各 DID 空間内に DeviceStorage（全データ + ChangeLog）と SharedStorage（共有レコード）を配置。インターフェース化されており、SQLite 参照実装またはアプリ独自の実装を注入できる。
- **アプリID**: アプリごとのデータは DID 空間内 `apps/<app-id>/` に分離。GroupShare の Channel 登録時にもアプリID で名前空間を分けることが可能。

---

## 6. 実装に必要な作業（Phase 2 で想定）

### 6.0 Phase 2 の実装順序

- **推奨順序（A）**: **LinkSelf データルート／DID 空間の決定** → **つながり（contacts, friend_requests, groups）の LinkSelf 内包** → **sync-db クライアント（アプリ向け DB インターフェース）** の順で着手する。
- **別案（B）**: つながりの内包を先に進める場合は、既存 daemon を拡張して contacts／groups の RPC を追加し、そのあとでデータルート／DID 空間を整理する進め方もあり得る。計画では A を主順序とし、B は段階的に移行する場合の選択肢として記述する。

### 6.1 LinkSelf データルートと DID 空間の決定

- **コアまたは daemon** で「LinkSelf データルート」を返す／受け取る仕様にする。
  - デフォルト: 上記のプラットフォーム別パス（Windows: AppData、macOS: Application Support、Linux: XDG）。
  - オプション: 環境変数や起動引数で `LINKSELF_DATA_DIR` を指定可能にする。
- **DID 空間**: データルートの下に `<DID>` ごとのディレクトリを用意する。DID はファイルシステムで安全な形（例: ハッシュやエンコード）に変換してディレクトリ名にする。
- daemon の `start` では **利用する DID**（または既存の `identityPath`）と **アプリID** を受け取り、**その DID の空間**および **その DID 空間内の `apps/<app-id>/`** を今回のセッションで使う。アプリID は sync-db やアプリごとのデータにアクセスする際の名前空間として使う。省略時は「既定 DID」や「一覧の先頭」などアプリ層の仕様に合わせる。

### 6.2 友だち・グループを LinkSelf に内包（DID 空間内）

- **保存先**: 選択された **DID 空間内**。例: `user.db`（SQLite3）に `contacts` / `friend_requests` / `groups` テーブル。
- **実装場所**: core 内（例: `core/internal/userstore` または既存の group 拡張）。daemon は「今回のセッションで使う DID」に応じてその DID 空間のストアを開き、RPC で getContacts / addContact / getFriendRequests / getGroups / createGroup 等を提供。
- **チャットクライアント**: 起動時または設定で **DID を選択**し、contacts / friend_requests の取得・更新は **その DID を指定した daemon の RPC** に切り替える。自前の JSON 読み書きは廃止。

### 6.3 チャット内容を DeviceSync + GroupShare で扱う（DID 空間内）

- **DeviceSync**: contacts / friend_requests / メッセージ履歴は `DeviceDB.Put` で DID 空間内に保存。同一 DID の全デバイスに自動複製される。
- **GroupShare**: チャットメッセージの送信は `GroupShare.Put("chat", msgID, body)` で Channel 経由。グループメンバーに AccessPolicy に沿って配信される。
- **受信側**: GroupShare の `SetOnSharedData` で受信 → ローカルの DeviceDB にも保存（履歴の全デバイス同期）。
- チャットクライアントは、メッセージ一覧の取得を **DeviceDB 経由の RPC**（ローカルから読み取り）、送信を **GroupShare 経由の RPC** に切り替える。

### 6.4 複数インスタンス・複数アプリと DID 選択

- **チャットクライアントを複数インスタンスで立ち上げる場合**: 各インスタンスで **DID を選択**する（起動引数・設定画面・起動時ダイアログなど）。選択した DID に紐づく LinkSelf データ（contacts、groups、sync.db）だけが使われる。
- **複数アプリ**: 同じ LinkSelf データルートを参照し、各アプリが「利用する DID」と **アプリID**（初期化時に渡す）を指定する。同じ DID を選べば、その DID のつながりをどのアプリからも利用できる。**アプリごとのデータ**は `LinkSelf/<DID>/apps/<app-id>/` のように DID 空間の下にアプリID で分離する。アプリID は衝突を避けるため、逆ドメインなどアプリ側で一意なものを渡す。
- 保存されている実存の一覧（利用可能な DID のリスト）を返す API を LinkSelf が提供し、アプリ層で「どれを使うか」を選べるようにする。

---

## 7. 依存・注意点

- **DeviceStorage / SharedStorage の SQLite 実装**: [sync-db 計画](sync-db-plan.md) に基づき、DeviceSync 用の DeviceStorage と GroupShare 用の SharedStorage の SQLite 実装を追加する。
- **DID のディレクトリ名**: DID 文字列はファイルシステムで安全な形（例: SHA256 の先頭バイトを hex、または base64url エンコード）に変換して DID 空間のディレクトリ名にする必要がある。
- **アプリID**: アプリごとのフォルダ名として使うアプリID は、**LinkSelf を利用するアプリがインスタンス初期化時（sync-db 等にアクセスするとき）に渡す**。LinkSelf は中央で ID を発行しない。アプリは Android のパッケージ名のように衝突しない一意な ID（例: 逆ドメイン `com.example.chat-client`）を選ぶ責任を持つ。アプリID をファイルシステムで安全なディレクトリ名に変換する必要がある場合は、DID と同様にエンコードする。
- **保存されている実存の一覧**: LinkSelf が「利用可能な DID のリスト」を返す API を提供し、アプリ層で起動時や設定画面で DID を選択できるようにする。
- **新規 DID 作成**: 新規 DID（新規実存）を作成する API／フローを Phase 2 で検討する。利用可能な DID 一覧に加え、ユーザーが新しい実存を追加する操作を LinkSelf が提供する。
- **同一 DID 空間への複数プロセス**: 複数アプリの daemon が同一 DID 空間のファイルに同時アクセスする場合の方針（SQLite の WAL モードや単一 daemon 共有など）を Phase 2 で検討する。
- **一覧取得**: DeviceStorage.List / SharedStorage.ListByChannel で一覧取得をサポート済み。
- **Windows AppData**: 実際のパスは `process.env.LOCALAPPDATA`（Node）や Go の `os.UserConfigDir` 等で取得する。インストーラやドキュメントで「LinkSelf のデータは AppData に保存されます」と明記するとよい。
