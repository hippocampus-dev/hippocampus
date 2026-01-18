# Bot

<!-- TOC -->
* [Bot](#bot)
  * [Development](#development)
    * [Bulk send messages](#bulk-send-messages)
    * [Emulate Slack event](#emulate-slack-event)
    * [Raise Rate Limit](#raise-rate-limit)
<!-- TOC -->

## Development

```sh
$ export SLACK_BOT_TOKEN=<YOUR SLACK BOT TOKEN>
$ export SLACK_SIGNING_SECRET=<YOUR SLACK SIGNING SECRET>
$ export OPENAI_API_KEY=<YOUR OPENAI API KEY>
$ make dev
```

### Bulk send messages

```sh
$ uv run -- python slack_message_event.py --channel <Slack Channel ID> --messages "<@U02CMRJA1GQ> hi" "<@U02CMRJA1GQ> hi"
```

### Emulate Slack event

```sh
$ uv run -- python slack_message_event.py --url <Slack "Copy link">
```

### Raise Rate Limit

```sh
$ uv run -- python slack_raise_rate_limit.py --url <Slack "Copy link">
```
