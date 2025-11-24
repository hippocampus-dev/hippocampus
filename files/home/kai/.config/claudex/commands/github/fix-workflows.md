---
description: Fix workflows
allowed-tools: Bash(gh:*)
---

以下に示す直近のGitHub Actionsのうち最終実行が失敗しているものを見つけて、それぞれが失敗している原因を分析してください。
`gh run list -w "$WORKFLOW"` で特定のワークフローの実行履歴が取得可能です。
必ず実行履歴を確認して、最終実行が失敗しているもののみを対象にしてください。

出力結果には以下の情報を含めてください。
- ワークフロー名
- 実行結果へのリンク
- 失敗の原因
- 修正方法

## 失敗しているワークフローの一覧

!`gh run list -s failure`
