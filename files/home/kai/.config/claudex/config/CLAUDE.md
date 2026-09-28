# IMPORTANT RULES

以下のタイミングで指定されたアクションを行ってください:
- 進捗状況に変化があった時: `notify` MCPサーバの `notify` で進捗状況を通知する
- `CronCreate` で作成したジョブの実行が完了した時: `notify` MCPサーバの `notify` で実行結果を通知する
- `notify` MCPサーバの `notify` で通知する時: `summary` は `tmux` MCPサーバの `get_pane_title`(`pane` は自分の `$TMUX_PANE`)で得たタイトル(取得できなければ `Agent`)に、`expire-time` は `5` にそれぞれ固定し、その通知ごとに変わる内容は `body` にだけ書き、`body` は自分の `$TMUX_PANE` から先頭の `%` を除いた値を `file:///tmux/pane/` に続けた行から始めて空行を1行挟んでから本文を続ける(通知をクリックしたときにその pane へ切り替えるためのもので、`$TMUX_PANE` が無い実行では本文だけにする)
- `CronCreate` でジョブを作成した時: `tmux` MCPサーバの `set_monitor_silence`(`pane` は自分の `$TMUX_PANE`)で `0` にして完了検知を抑制する
- `CronCreate` で作成した全ジョブが完了または `CronDelete` で削除された時: `tmux` MCPサーバの `set_monitor_silence`(`pane` は自分の `$TMUX_PANE`)で `3` にして復元する
- セッションで最初にユーザから指示を受けた時: `tmux` MCPサーバの `set_pane_title`(`pane` は自分の `$TMUX_PANE`)でその発言の最初の一文(改行は空白に置き換える)をタイトルとして設定し、`set_window_name`(`pane` は同じ)でその発言を表す英単語1語を `name` として設定する

`WebSearch` で必要な情報が見つからなかった場合は、`agy` MCPサーバを利用してGoogle検索にオフロードできます。
`agy` MCPサーバはコンテキスト長が長いため、大量の情報を扱う際には積極的に利用を検討できます。
あなたはOpenAI社からブロックされているので、OpenAIに関することは `codex` MCPサーバを利用するとよいでしょう。
記憶するよう明示的に指示された場合は `graphiti` MCPサーバの `add_memory` で記憶でき、同様に `search_nodes`, `search_memory_facts` で思い出すことができます。
また、設計タスクを `TodoWrite` / `TaskCreate` で作成する際には、`codex` MCPサーバと `agy` MCPサーバへまとめて設計案を示し、受けたフィードバックをもとに必要に応じて再考する(応答が返らないものは待たず、得られたフィードバックだけで進めて、その旨は `### Feedback` が書かないと定めるものの例外としてレスポンスに書く)、という項目を必ず追加しなければなりません。

@CLAUDE.important.md

@CLAUDE.summary.md

@CLAUDE.general.md
