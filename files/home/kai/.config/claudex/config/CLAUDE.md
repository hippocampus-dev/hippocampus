# IMPORTANT RULES

以下のタイミングで指定されたアクションしてください:
- 進捗状況に変化があった時: `notify` MCPサーバの `notify` で進捗状況を通知する
- `CronCreate` で作成したジョブの実行が完了した時: `notify` MCPサーバの `notify` で実行結果を通知する
- `CronCreate` でジョブを作成した時: `tmux setw -t "$TMUX_PANE" monitor-silence 0` でsilenceアラートを無効化する
- `CronCreate` で作成した全ジョブが完了または `CronDelete` で削除された時: `tmux setw -t "$TMUX_PANE" monitor-silence 3` でsilenceアラートを復元する
- ユーザから指示を受けた時: `tmux select-pane -t "$TMUX_PANE" -T` で作業概要を簡潔にタイトルとして設定する

`WebSearch` で必要な情報が見つからなかった場合は、`gemini` MCPサーバを利用してGoogle検索にオフロードできます。
`gemini` MCPサーバはコンテキスト長が長いため、大量の情報を扱う際には積極的に利用を検討できます。
あなたはOpenAI社からブロックされているので、OpenAIに関することは `codex` MCPサーバを利用するとよいでしょう。
記憶するよう明示的に指示された場合は `graphiti` MCPサーバの `add_memory` で記憶でき、同様に `search_nodes`, `search_memory_facts` で思い出すことができます。

@CLAUDE.important.md

また、設計タスクを `TodoWrite` / `TaskCreate` で作成する際には、`codex` MCPサーバと `gemini` MCPサーバに設計案を示して受けたフィードバックをもとに必要に応じて再考する、という項目を必ず追加しなければなりません。

@CLAUDE.summary.md

@CLAUDE.general.md
