# API

## Development

```sh
$ poetry install
$ poetry run -- playwright install chromium
```

After preparation, do the following.

```sh
$ export OPENAI_API_KEY=<YOUR OPENAI API KEY>
$ poetry run -- python api/main.py
```

### How to debug OpenAI API

Access mitmweb: http://127.0.0.1:18081
