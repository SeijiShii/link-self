# ロードマップ修正計画書（実施済み）

**実施日:** 2026-02-08  
**ステータス:** 実施済み  
**概要:** ロードマップ修正に基づき、OSS・Gitcoin・本格サンプルアプリ（LinkSelf をインフラとして使用、グループ情報等は LinkSelf 側 DB）およびライブラリ利用の道筋をドキュメント全体に反映した。

**参照:** [README.ja.md](README.ja.md#roadmap)、[sample-chat-app-plan.md](sample-chat-app-plan.md)

---

## 反映した変更

### ロードマップ・README

- **README.md:** Phase 1 を Done に。Phase 2 を「1 本の本格サンプルアプリ（チャット＋ファイル共有）が LinkSelf をインフラとして使用；他アプリから LinkSelf をライブラリとして使う道筋の整備」に変更。Open Source（Apache-2.0）と Support / Donate（Gitcoin 予定）の短文を追加。
- **docs/README.en.md:** Open Source と Support / Donate セクションを追加。Phase 2 を上記方針に合わせて更新。Sample app plan の参照を「Sample app plan (chat + file sharing)」に。
- **docs/README.ja.md:** 同上（日本語）。目次に Open Source・Donate を追加。サンプルアプリ計画のリンク表記を「サンプルアプリ計画（チャット＋ファイル共有）」に。

### サンプルアプリ計画

- **docs/sample-chat-app-plan.en.md / .md:** タイトルを「サンプルアプリ計画（チャット＋ファイル共有）」に。概要を「1 本のアプリが LinkSelf をインフラとして使用；グループ情報等は LinkSelf 側の DB」に変更。「役割とデータ」セクションを追加。ファイル共有のスコープと追加機能を記載。Summary/まとめを更新。
- **docs/phase1-design.en.md / .md:** `cmd/linkself` の説明を「サンプルアプリ用エントリ（チャット＋ファイル共有、未実装）」に変更。

### 今後の Phase 2 実装時

- **ライブラリ利用の道筋:** Phase 2 で `docs/using-linkself-as-library.md`（および .en.md）を追加し、他アプリから LinkSelf を組み込む手順（公開 API・パッケージング・利用例）を記載する。
- **オプション:** CONTRIBUTING.md、CODE_OF_CONDUCT.md の追加。

---

## Gitcoin で資金調達するために必要なこと（参考）

ロードマップに Gitcoin を掲げるにあたり、実際に資金調達するまでに必要な作業の要約。

- **流れ:** (1) Builder でプロジェクト（グラントページ）を作成 → (2) Explorer で該当ラウンドに申請（ガス代が必要）。
- **用意するもの:** ウォレット（ガス代）、ロゴ 300×300 px・バナー 1500×500 px、説明文（課題・解決策・資金使途・チーム）、Twitter/X と公開 GitHub の検証。
- **資格の目安:** オープンソース・Permissive ライセンス（本プロジェクトは Apache-2.0 で適合）、開発実績（初回コミット 90 日以上前、直近 30 日以内のコミット等のうち 3 つ以上）。禁止事項: 差別・詐欺・Sybil・寄付見返り・法規違反。
- **ラウンド:** 四半期ごと等。[grants-portal.gitcoin.co](https://grants-portal.gitcoin.co/gitcoin-grants-grantee-portal) 等で申請時期を確認。

---

以上。
