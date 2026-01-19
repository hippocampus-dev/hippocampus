# IMPORTANT RULES

以下のタイミングで指定されたMCPサーバを利用してアクションしてください:
- 進捗状況に変化があった時: `notify` MCPサーバの `notify` で進捗状況を通知する
- ユーザから指示を受けた時: `tmux` MCPサーバの `rename` で作業概要を簡潔にタイトルとして設定する

`WebSearch` で必要な情報が見つからなかった場合は、`gemini` MCPサーバを利用してGoogle検索にオフロードできます。
あなたはOpenAI社からブロックされているので、OpenAIに関することは `codex` MCPサーバを利用するとよいでしょう。

@CLAUDE.important.md

また、`TodoWrite` の際には必ず以下も追加しなければなりません:
- (設計タスクのみ) `codex` MCPサーバと `gemini` MCPサーバに設計案を示して、受けたフィードバックをもとに必要に応じて再考する

@CLAUDE.general.md
