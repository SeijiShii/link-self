# Gitcoin で資金調達するための手順

**日本語**（このページ）| [English](gitcoin-funding.en.md)  
**概要:** 本プロジェクトを Gitcoin Grants で寄付・マッチングの対象とするために必要な手順をまとめたドキュメント。  
**参照:** [ロードマップ](README.ja.md#roadmap)、[ロードマップ修正計画書](roadmap-revision-plan.md)

---

## 1. 全体の流れ（2 段階）

Gitcoin Grants に参加するには、次の **2 つ** を別々に行う必要があります。

1. **Builder でプロジェクト（グラントページ）を作成する** — プロジェクトの説明・画像・検証を登録し、オンチェーンで公開する。
2. **Explorer で該当ラウンドに申請する** — ラウンド開催時に、作成したプロジェクトでそのラウンドに「申請」する。ラウンドごとにガス代が必要。

どちらも **ウォレット接続** と **ガス代**（チェーン手数料）が必要です。

---

## 2. Builder でプロジェクトを作成する手順

### 2.1 アクセスとウォレット接続

- [builder.gitcoin.co](https://builder.gitcoin.co/) にアクセスする。
- **「Connect Wallet」** をクリックし、MetaMask 等のウォレットを接続する。
- ウォレットがない場合は「Get a Wallet」で作成する。
- **注意:** 編集権限は **1 アカウントのみ**。プロジェクトを更新する人が接続するウォレットを選ぶ。

### 2.2 新規プロジェクトの作成

- **「+ New Project」** または **「Create a Project」** をクリックする。
- **ネットワーク（チェーン）を選択する。** グラントページをデプロイするチェーンを選ぶ。**そのチェーンにガス代用のトークンが必要**なので、手持ちのネットワークを選ぶ。

### 2.3 プロジェクト詳細の入力

次の項目を入力する。

| 項目 | 要件・推奨 |
|------|------------|
| **ロゴ** | 300×300 px |
| **バナー** | 1500×500 px |
| **説明文** | 見出しと段落で整理。推奨: **Problem（課題）**、**Solution（解決策）**、**Updates（前回以降の進捗）**、**Plans for Funding（資金の使い道）**、**Team Members（チーム）**。写真や実績・インパクトの証拠があるとよい。Markdown 使用可。 |

### 2.4 ソーシャル・GitHub の検証

- **Twitter/X:** プロジェクト用アカウントでログインした状態で別タブを開き、Builder の認証フローで連携する。
- **GitHub:** 組織の場合は **公開プロフィール** が必要。認可して検証する。

### 2.5 保存と公開

- プレビューを確認し、**「Save and Publish」** をクリックする。
- ウォレットでガス代を支払う。**編集のたびに「Save and Publish」するとガス代がかかる**（オンチェーン取引のため）。
- 「Project Created!」と表示されればプロジェクト作成完了。

### 2.6 ラウンドへの申請

- プロジェクト作成後、**Explorer**（[explorer.gitcoin.co](https://explorer.gitcoin.co/) 等）で該当ラウンドを開く。
- そのラウンドに「申請」する。**ラウンドがデプロイされているチェーンにガス代**が必要。
- ラウンドごとに申請期間があるため、時期は [grants-portal.gitcoin.co](https://grants-portal.gitcoin.co/gitcoin-grants-grantee-portal) や Gitcoin の Telegram・メール登録で確認する。

---

## 3. 用意するもの一覧

| 項目 | 内容 |
|------|------|
| **ウォレット** | MetaMask 等。ガス代を払えるネットワークに少額のネイティブトークン（例: ETH）を用意する。 |
| **画像** | ロゴ 300×300 px、バナー 1500×500 px |
| **説明文** | 課題・解決策・進捗・資金使途・チーム（Markdown 可）。見出しと段落で読みやすくする。 |
| **検証** | プロジェクトの Twitter/X アカウント、**公開**の GitHub 組織またはリポジトリ |
| **ガス代** | プロジェクト作成・編集時、およびラウンド申請時のチェーン手数料 |

---

## 4. 参加資格（Eligibility）の目安

Gitcoin Grants では、プロジェクトの参加資格が審査される。以下は一般的な目安。

### 4.1 オープンソース

- ソースコードが **公開** され、誰でもフォーク・改変・再配布できること。
- **Permissive なライセンス**（Apache-2.0、MIT など）が望ましい。本プロジェクトは **Apache-2.0** で適合。
- 非 Permissive の場合は手動審査の対象となることがある。

### 4.2 開発実績（Activity）

次の **4 つのうち 3 つ以上** を満たすとスムーズ。**2 つ** なら手動審査。**2 つ未満** は原則不適格の可能性あり。

- 初回コミットが **90 日以上前**
- **直近 30 日以内** にコミットがある
- **過去 90 日で 20 日以上** 活動している
- **2 人以上** がコントリビュートしている

### 4.3 禁止事項

- 差別・ヘイト、ユーザーを欺く内容
- Sybil 攻撃などの不正、なりすまし
- 寄付と引き換えの見返り（Quid Pro Quo）
- 広告・販促目的のみの利用
- 法規違反

---

## 5. ラウンドのタイミング

- Gitcoin Grants は **四半期ごと** などのラウンドで実施される。
- **ラウンドごとに申請期間** がある。申請はその期間内に行う。
- 最新のラウンド情報・申請時期は次で確認する。
  - [Gitcoin Grants for Grantees（グランティーポータル）](https://grants-portal.gitcoin.co/gitcoin-grants-grantee-portal)
  - [Gitcoin Telegram](https://t.me/+W9y5_KplLX4xYjcx)
  - Gitcoin メール登録（ポータル内の「Subscribe to emails」等）

---

## 6. 本プロジェクト（LinkSelf）での対応状況

- **ライセンス:** Apache-2.0（Permissive）— 資格を満たす。
- **寄付ページ:** 未作成。README では「Gitcoin Grants でプロジェクトを登録し寄付を募る予定」と記載し、URL は準備でき次第追記する方針。
- **実施時:** 本ドキュメントの手順に沿って Builder でプロジェクトを作成し、ラウンド開催時に Explorer で申請する。

---

## 7. 参考リンク

- [Builder（プロジェクト作成）](https://builder.gitcoin.co/)
- [Gitcoin Grants for Grantees（グランティーポータル）](https://grants-portal.gitcoin.co/gitcoin-grants-grantee-portal)
- [Gitcoin General Eligibility Criteria（参加資格）](https://grants-portal.gitcoin.co/gitcoin-grants-grantee-portal/gitcoin-general-eligibility-criteria)
- [Gitcoin Support（Builder ガイド等）](https://support.gitcoin.co/gitcoin-knowledge-base/gitcoin-grants-program/builder-a-guide-for-grantees)
- 問い合わせ: support@gitcoin.co
