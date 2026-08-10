# IMPORTANT RULES

以下のタイミングで指定されたアクションを行ってください:
- 進捗状況に変化があった時: `notify` MCPサーバの `notify` で進捗状況を通知する
- `CronCreate` で作成したジョブの実行が完了した時: `notify` MCPサーバの `notify` で実行結果を通知する
- `CronCreate` でジョブを作成した時: `tmux` MCPサーバの `set_monitor_silence`(`pane` は自分の `$TMUX_PANE`)で `0` にして完了検知を抑制する
- `CronCreate` で作成した全ジョブが完了または `CronDelete` で削除された時: `tmux` MCPサーバの `set_monitor_silence`(`pane` は自分の `$TMUX_PANE`)で `3` にして復元する
- ユーザから指示を受けた時: `tmux` MCPサーバの `set_title`(`pane` は自分の `$TMUX_PANE`)で作業概要を簡潔にタイトルとして設定する

`WebSearch` で必要な情報が見つからなかった場合は、`gemini` MCPサーバを利用してGoogle検索にオフロードできます。
`gemini` MCPサーバはコンテキスト長が長いため、大量の情報を扱う際には積極的に利用を検討できます。
あなたはOpenAI社からブロックされているので、OpenAIに関することは `codex` MCPサーバを利用するとよいでしょう。
記憶するよう明示的に指示された場合は `graphiti` MCPサーバの `add_memory` で記憶でき、同様に `search_nodes`, `search_memory_facts` で思い出すことができます。
また、設計タスクを `TodoWrite` / `TaskCreate` で作成する際には、`codex` MCPサーバと `gemini` MCPサーバへまとめて設計案を示し、受けたフィードバックをもとに必要に応じて再考する、という項目を必ず追加しなければなりません。

@CLAUDE.important.md

@CLAUDE.summary.md

@CLAUDE.general.md
