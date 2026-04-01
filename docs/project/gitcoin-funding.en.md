# Gitcoin funding procedure

**English** (this page) | [日本語](gitcoin-funding.md)  
**Summary:** Steps required to register this project on Gitcoin Grants for donations and matching.  
**See also:** [Roadmap](README.en.md#roadmap), [Roadmap revision plan](roadmap-revision-plan.en.md)

---

## 1. Overview (two steps)

To participate in Gitcoin Grants, you must do **two** things:

1. **Create your project (grant page) on Builder** — Register project description, images, and verification; publish on-chain.
2. **Apply to a round on Explorer** — When a round is open, apply to that round with your project. Gas fees are required per round.

Both require **wallet connection** and **gas** (chain fees).

---

## 2. Creating a project on Builder

### 2.1 Access and wallet connection

- Go to [builder.gitcoin.co](https://builder.gitcoin.co/).
- Click **“Connect Wallet”** and connect a wallet (e.g. MetaMask).
- If you don’t have a wallet, use “Get a Wallet” to create one.
- **Note:** Only **one account** has edit access. Use the wallet of the person who will update the project.

### 2.2 Create a new project

- Click **“+ New Project”** or **“Create a Project”**.
- **Select the network (chain)** where the grant page will be deployed. You need **gas on that chain**, so choose a network where you have funds.

### 2.3 Project details

Fill in:

| Item | Requirement / recommendation |
|------|------------------------------|
| **Logo** | 300×300 px |
| **Banner** | 1500×500 px |
| **Description** | Use headings and paragraphs. Suggested: **Problem**, **Solution**, **Updates** (since last round), **Plans for Funding**, **Team Members**. Photos and proof of work/impact help. Markdown OK. |

### 2.4 Verify social and GitHub

- **Twitter/X:** Have the project’s Twitter account logged in in another tab; complete the verification flow in Builder.
- **GitHub:** For organizations, a **public profile** is required. Authorize and verify.

### 2.5 Save and publish

- Review the preview and click **“Save and Publish”**.
- Pay the gas fee in your wallet. **Each time you edit and “Save and Publish,” you pay gas** (on-chain transaction).
- When you see “Project Created!”, the project is created.

### 2.6 Apply to a round

- After creating the project, open **Explorer** (e.g. [explorer.gitcoin.co](https://explorer.gitcoin.co/)) and find the round you want.
- **Apply** to that round. **Gas on the round’s chain** is required.
- Rounds have application windows; check [grants-portal.gitcoin.co](https://grants-portal.gitcoin.co/gitcoin-grants-grantee-portal), Gitcoin Telegram, or email signup for timing.

---

## 3. What to prepare

| Item | Details |
|------|---------|
| **Wallet** | e.g. MetaMask, with a small amount of native tokens (e.g. ETH) for gas on the chosen network. |
| **Images** | Logo 300×300 px, banner 1500×500 px |
| **Copy** | Problem, solution, updates, plans for funding, team (Markdown OK). Use headings and paragraphs. |
| **Verification** | Project Twitter/X account; **public** GitHub org or repo |
| **Gas** | For project create/edit and for round application |

---

## 4. Eligibility (summary)

Gitcoin reviews projects for eligibility. General guidelines:

### 4.1 Open source

- Source code **available**; anyone can fork, modify, and redistribute.
- **Permissive license** (Apache-2.0, MIT, etc.) preferred. This project uses **Apache-2.0** and qualifies.
- Non-permissive licenses may be subject to manual review.

### 4.2 Development activity

Meeting **3 or more** of the following is typical; **2** may get manual review; **fewer than 2** may be ineligible.

- First commit **90+ days ago**
- **Commit in the last 30 days**
- **20+ active days in the last 90 days**
- **2+ contributors**

### 4.3 Prohibited

- Hate, discrimination, deceiving users
- Sybil attacks, fraud, impersonation
- Quid-pro-quo for donations
- Purely advertising/promotional use
- Legal violations

---

## 5. Round timing

- Gitcoin Grants runs in **rounds** (e.g. quarterly).
- Each round has an **application window**. Apply during that window.
- Check for current rounds and deadlines:
  - [Gitcoin Grants for Grantees](https://grants-portal.gitcoin.co/gitcoin-grants-grantee-portal)
  - [Gitcoin Telegram](https://t.me/+W9y5_KplLX4xYjcx)
  - Email subscription (e.g. “Subscribe to emails” on the portal)

---

## 6. This project (LinkSelf)

- **License:** Apache-2.0 (permissive) — qualifies.
- **Grant page:** Not yet created. README states we plan to register on Gitcoin Grants; URL will be added when ready.
- **When applying:** Follow this document to create the project on Builder and apply to a round on Explorer when it opens.

---

## 7. References

- [Builder (create project)](https://builder.gitcoin.co/)
- [Gitcoin Grants for Grantees](https://grants-portal.gitcoin.co/gitcoin-grants-grantee-portal)
- [Gitcoin General Eligibility Criteria](https://grants-portal.gitcoin.co/gitcoin-grants-grantee-portal/gitcoin-general-eligibility-criteria)
- [Gitcoin Support (Builder guide, etc.)](https://support.gitcoin.co/gitcoin-knowledge-base/gitcoin-grants-program/builder-a-guide-for-grantees)
- Contact: support@gitcoin.co
