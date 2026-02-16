# IMPORTANT RULES

以下のタイミングで指定されたMCPサーバを利用してアクションしてください:
- 進捗状況に変化があった時: `notify` MCPサーバの `notify` で進捗状況を通知する
- ユーザから指示を受けた時: `tmux` MCPサーバの `rename` で作業概要を簡潔にタイトルとして設定する

`WebSearch` で必要な情報が見つからなかった場合は、`gemini` MCPサーバを利用してGoogle検索にオフロードできます。
`gemini` MCPサーバはコンテキスト長が長いため、大量の情報を扱う際には積極的に利用を検討できます。
あなたはOpenAI社からブロックされているので、OpenAIに関することは `codex` MCPサーバを利用するとよいでしょう。

@CLAUDE.important.md

また、`TodoWrite` の際には必ず以下も追加しなければなりません:
- (設計タスクのみ) `codex` MCPサーバと `gemini` MCPサーバに設計案を示して、受けたフィードバックをもとに必要に応じて再考する

記憶するよう明示的に指示された場合は `graphiti` MCPサーバの `add_memory` で記憶でき、同様に `search_nodes`, `search_memory_facts` で思い出すことができます。

@CLAUDE.general.md
