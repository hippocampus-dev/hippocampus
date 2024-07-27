# Bot

## Development

```sh
$ poetry install
$ poetry run -- playwright install chromium
```

After preparation, do the following.

```sh
$ export SLACK_BOT_TOKEN=<YOUR SLACK BOT TOKEN>
$ export SLACK_SIGNING_SECRET=<YOUR SLACK SIGNING SECRET>
$ export OPENAI_API_KEY=<YOUR OPENAI API KEY>
$ poetry run -- python bot/main.py
```

### Emulate Slack event

```sh
$ poetry run -- python slack_message_event.py --url <Slack "Copy link">
```

### How to debug OpenAI API

Access mitmweb: http://127.0.0.1:18081
