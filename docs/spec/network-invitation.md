# ネットワーク招待（Network Invitation）

**参照:** [ネットワークの概念](network-concept.md)、[データ同期設計](sync-db-plan.md)、[ブラウザ / PWA 対応](browser-pwa-support.md)

既存ネットワークに **新しい DID を参加させる**ための、URL / QR で配布できる時間制限付き招待の設計。home-visit-suite の「他のユーザーをグループに招待する」機能（URL 送付・QR 手渡し・トークン期限 3 日）を実現する、LinkSelf 側のプロトコル。

> **状態:** Slice 1（招待トークン `invitation.ts`）・Slice 2（参加ハンドシェイク `join.ts` + live 配線）・Slice 3（メンバーシップ伝播 `network-meta.ts` + 被招待者 bootstrap）実装済み・e2e GREEN（TS-first）。Slice 4（PWA 統合）は未着手。

---

## 1. デバイスペアリングとの違い

| | デバイスペアリング（`pairing.ts`） | ネットワーク招待（`invitation.ts`） |
|---|---|---|
| 目的 | 同一ユーザーの別端末を追加 | **別 DID の新メンバー**を参加させる |
| 運ぶもの | ユーザー秘密鍵（libp2p protobuf） | **鍵は運ばない**。署名付き権限（capability）のみ |
| 結果 | 同一 DID・同一データ空間 | 招待者と別 DID。ネットワークにメンバー追加 |
| 既定 TTL | 数分（同席で即時） | 3 日（後日ブラウザで開く運用） |

招待は**秘密鍵を一切含まない**。被招待者は自分の identity を持つ（無ければ参加時に作成）。

---

## 2. 招待トークン（Slice 1・実装済み）

管理者（ネットワークの AdminRole を持つメンバー）が自分のユーザー鍵で claims に署名して発行する。

### 署名対象 claims（`InviteClaims`）

| フィールド | 意味 |
|---|---|
| `v` | バージョン（1） |
| `networkId` | 参加先ネットワーク |
| `suiteId` | アプリ／スイート識別子（クロスアプリ replay 防止） |
| `role` | 受理時に割り当てるロール（ロール DAG で解決） |
| `inviterDID` | 発行者 DID（＝署名者・被招待者が認証する相手） |
| `nonce` | 招待の一意 ID（32 hex）。単回使用追跡・失効の handle |
| `expiresAt` | 失効時刻（epoch ms） |

### 署名対象外（envelope）

- `sig` — 発行者による Ed25519 署名（canonical JSON、固定フィールド順）
- `relays` — 到達性ヒント（circuit-relay multiaddr）。**署名対象外**（差し替わっても E2E 内容は読めず、被招待者はハンドシェイクで `inviterDID` を認証するため権限には無関係）

### API

- `createInvite(inviter, params, ttlMs, now?) → Invite` — 署名付き招待を発行
- `verifyInvite(invite, now?)` — 署名と期限を検証（`InviteError`: `invite_expired` / `invite_invalid`）。**発行者が実際に管理者かは検証しない**（受理ノードがローカルのネットワーク状態に対して確認する責務）
- `encodeInvite` / `decodeInvite` — base64url JSON 符号化（URL / QR 用）
- `buildInviteUrl(base, invite)` → `${base}/#/join?i=...`（フラグメント＝サーバー非送出）
- `extractInviteParam(input)` — URL / フラグメント / 生ペイロードから `i=` を取り出す

---

## 3. 参加ハンドシェイク（Slice 2・実装済み）

ネットワークのメンバーリストは**各ノードがローカル保持**（network-concept.md §1-2、DHT に実体なし）。したがって受理は**管理者のノード上**で起きる。

```
被招待者                              管理者（inviterDID or 任意の Admin メンバー）
  │  URL/QR から Invite を取得
  │  identity を用意（無ければ作成）
  │  relays 経由で inviterDID へ dial（circuit-relay v2）
  │  LinkSelf auth（相手 DID を認証）
  ├─ open /linkself/join/1.0.0 ───────▶
  │   { invite, inviteeDID, displayName }
  │                                     verifyInvite(invite)                （署名・期限）
  │                                     invite.inviterDID が自分の Admin か  （ローカル network 状態）
  │                                     nonce 未使用か                       （単回使用）
  │                                     NetworkService.addMember(networkId, self, inviteeDID, invite.role)
  │◀──────────────── { network snapshot, groupShare bootstrap } ──
  │  ローカルに Network を保存し GroupShare 購読を開始
  ▼  以後スイートのデータが同期される
```

- **受理者**は `inviterDID` 本人でなくてよい（任意の Admin メンバーが発行者署名を検証できる）。ただし単回使用（nonce 消費）と失効は各ノードでの追跡が必要。
- 応答にネットワークスナップショット（members + roles）と GroupShare のチャンネル bootstrap を含め、被招待者が即同期に入れるようにする。

### 実装ノート（Slice 2b）

- **別 auth ストリームは張らない。** 接続確立時の noise ハンドシェイクが相手の transport 鍵を暗号学的に証明するため、単一デバイス（`peerId ≡ アカウント DID`）では受理ノードが `remoteDID(connection)` で被招待者の DID を得られる。受理側は request 内の自己申告 `inviteeDID` ではなく**この transport 認証済み DID** をメンバーにバインドする（他人の DID を騙れない）。
- **既知の限界:** 2 層 identity（device 鍵とアカウント鍵が分離した既存マルチデバイスユーザー）が招待を受ける場合、transport 鍵は device DID なのでバインドがずれる。新規参加は単一デバイスから始まる想定でありこのケースは後続（roster 提示付きハンドシェイクで解消）。
- 被招待者側での snapshot ローカル永続化（＝データ面への参加）は Slice 4。`MemNetworkStore.createNetwork` は id を自動採番するため、同一 `networkId` で upsert する経路は Slice 4 で追加する。

---

## 4. メンバーシップの伝播（Slice 3・実装済み）

複数の管理者がいる場合、参加を受理した管理者以外もメンバー変更を知る必要がある。ネットワークのメンバー／ロール状態自体を共有データとして配信して収束させる。

**実装（`network-meta.ts`）:**
- `network_meta` envelope 型（TS 先行。Go は未知型として message にフォールバックするため相互運用を壊さない）で `NetworkMeta{networkId, suiteId, members, memberRoles, epoch}` を全メンバーへブロードキャスト（`client.publishMembership`）。
- 受理 admin が addMember 後に自動 publish。受信側は **送信者が自ローカル状態で admin か**を検証してから適用（非 admin メンバーによる membership 偽造を防ぐ）。`NetworkMetaTracker` が epoch の LWW で `NetworkStore.putNetwork` に upsert。
- **被招待者の bootstrap:** 認証済み join ストリームで受け取った応答スナップショットを `client.requestJoin` 内でローカルへ適用（＝データ面へ参加）。以後の network_meta で収束する。GroupShare（チャンネルが単一 groupId 束縛）ではなく専用 envelope にしたのは、参加前の被招待者や複数ネットワークを跨ぐケースで束縛が合わないため。
- **残:** nonce 失効リストの配布、複数ネットワーク同時参加時の分離、Network への永続 epoch 保持（現状は tracker 内メモリ）。

---

## 5. TTL・失効・単回使用

- 既定 TTL は運用側が決める（home-visit-suite = 3 日）。
- `nonce` を受理ノードが記録し、二重受理を防ぐ（単回使用）。発行者は未受理の nonce を失効リストに載せて配布中の招待を取り消せる（Slice 3 の network-meta に相乗り可能）。
- 期限切れ・失効・改竄は `verifyInvite` と受理ノードのチェックで拒否。

---

## 6. Go パリティ

本プロトコルは PWA 需要ドリブンで **TS 先行**。canonical claims の JSON 正規化（固定フィールド順）を Go 側 daemon が join ハンドシェイクを持つ際にミラーすれば相互運用可能（groupshare interop と同方針、browser-pwa-support.md §7 参照）。
