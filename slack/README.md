# Slack

- `invoke-openai.ts`: Invoke OpenAI API.
- `open-google-apps-script.ts`: Open Google Apps Script.
- `open-github-file.ts`: Open GitHub file.
- `send_message_from_csv.ts`: Send a message to a channel from CSV. Each CSV row can be mapped to a template to send messages in batches.
- `reply_in_thread_from_csv.ts`: Reply to a message in thread from CSV. Each CSV row can be mapped to a template to send messages in batches.
- `retrieve_message.ts`: Retrieve message from a message link.
- `retrieve_message_from_reaction.ts`: Retrieve messages from reaction.

## Preparation

### Create Google OAuth Web Client ID

1. https://api.slack.com/tutorials/tracks/oauth-tutorial#prep-google-services
2. `export SLACK_AUTOMATION_GOOGLE_CLIENT_ID`

### Create GitHub OAuth Web Client ID

1. https://docs.github.com/ja/apps/oauth-apps/building-oauth-apps/creating-an-oauth-app
2. Add `https://oauth2.slack.com/external/auth/callback` to `Authorization callback URL`
3. Generate a new client secret
4. `export SLACK_AUTOMATION_GITHUB_CLIENT_ID`

### Export Slack OAuth Web Client ID

1. Add `https://oauth2.slack.com/external/auth/callback` to `Redirect URLs`
2. `export SLACK_AUTOMATION_SLACK_CLIENT_ID`

## Development

```sh
$ slack auth login
$ slack run
```

## Deployment

```sh
$ slack auth login
$ slack deploy
$ slack function distribute --name invoke-openai --everyone --grant
$ slack function distribute --name open-google-apps-script --everyone --grant
$ slack function distribute --name open-github-file --everyone --grant
$ slack function distribute --name send_message --everyone --grant
$ slack function distribute --name send_message_from_csv --everyone --grant
$ slack function distribute --name reply_in_thread_from_csv --everyone --grant
$ slack function distribute --name retrieve_message --everyone --grant
$ slack function distribute --name retrieve_message_from_reaction --everyone --grant
$ slack external-auth add-secret --provider google --secret $SLACK_AUTOMATION_GOOGLE_CLIENT_SECRET
$ slack external-auth add-secret --provider github --secret $SLACK_AUTOMATION_GITHUB_CLIENT_SECRET
$ slack external-auth add-secret --provider my_slack --secret $SLACK_AUTOMATION_SLACK_CLIENT_SECRET
```
