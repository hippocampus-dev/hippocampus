---
description: Generate weekly progress report from git commits (user)
allowed-tools: Bash(git:*)
---

Generate a comprehensive weekly progress report based on git commits from the past week.

## Commits from the past week

!`git log --since="1 week ago" --oneline --no-merges`

## Instructions

1. **Analyze commits**: Use `git log --since="1 week ago" --stat --format="%h %s"` to understand the scope and impact of each commit.

2. **Categorize commits** by area of work:
   - Kubernetes/Infrastructure deployments
   - Feature development (by component/package)
   - CI/CD and workflow improvements
   - Documentation updates
   - Bug fixes and refactoring
   - Configuration changes

3. **Focus on impact**: Describe what was accomplished, not just what files changed. Explain the purpose and benefit of significant changes.

4. **Be concise**: Aggregate similar changes (e.g., "12 Kubernetes service deployments" instead of listing each one individually).

## Output Template

Generate the report in Japanese using EXACTLY this format:

```markdown
# 週次進捗レポート (YYYY-MM-DD 〜 YYYY-MM-DD)

## サマリー
- **総コミット数**: N件
- **主要作業領域**: [カンマ区切りで3-5項目]

---

## カテゴリ別詳細

### 1. Kubernetes/インフラストラクチャ

[箇条書きで主要な変更を記載]

### 2. 機能開発

[コンポーネント/パッケージごとにサブセクションを作成]

### 3. CI/CD & ワークフロー

[箇条書きで主要な変更を記載]

### 4. オブザーバビリティ & 監視

[箇条書きで主要な変更を記載]

### 5. 開発環境 & ツール

[箇条書きで主要な変更を記載]

### 6. バグ修正 & リファクタリング

[箇条書きで主要な変更を記載]

### 7. ドキュメント & 設定

[箇条書きで主要な変更を記載]

---

## 主要な成果

1. **[成果タイトル]**: [1-2文で技術的な詳細と効果を説明]
2. **[成果タイトル]**: [1-2文で技術的な詳細と効果を説明]
3. **[成果タイトル]**: [1-2文で技術的な詳細と効果を説明]
[3-5項目]
```

Notes:
- Use `---` horizontal rules to separate major sections
- Use bold (`**text**`) for emphasis on important terms
- Each category section should have substantive content or be omitted if no relevant commits exist
- Keep bullet points concise (1-2 lines each)
- The "主要な成果" section should highlight the most impactful work of the week
