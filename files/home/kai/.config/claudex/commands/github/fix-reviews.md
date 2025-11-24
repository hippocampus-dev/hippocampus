---
description: Fix reviews
allowed-tools: Bash(gh:*)
---

以下に示すPullRequestの一覧のうちレビュー済みのものを見つけて、それぞれで必要な対応を分析してください。
`gh pr view "$PR_NUMBER"` で特定のPullRequestの詳細が取得可能です。

出力結果には以下の情報を含めてください。
- PullRequest番号
- レビュー内容
- 必要な対応

## PullRequestの一覧

!`gh pr list --author "@me"`
