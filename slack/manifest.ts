import {DefineOAuth2Provider, Manifest, Schema} from "deno-slack-sdk/mod.ts";
import {def as InvokeOpenAI} from "./functions/invoke-openai.ts";
import {def as OpenGoogleAppsScript} from "./functions/open-google-apps-script.ts";
import {def as OpenGitHubFile} from "./functions/open-github-file.ts";
import {def as SendMessage} from "./functions/send_message.ts";
import {def as SendMessageFromCSV} from "./functions/send_message_from_csv.ts";
import {def as ReplyMessageFromCSV} from "./functions/reply_in_thread_from_csv.ts";
import {def as RetrieveMessage} from "./functions/retrieve_message.ts";
import {def as RetrieveMessagesFromReaction} from "./functions/retrieve_messages_from_reaction.ts";

const GoogleProvider = DefineOAuth2Provider({
    provider_key: "google",
    provider_type: Schema.providers.oauth2.CUSTOM,
    options: {
        provider_name: "Google",
        authorization_url: "https://accounts.google.com/o/oauth2/auth",
        token_url: "https://oauth2.googleapis.com/token",
        client_id: Deno.env.get("SLACK_AUTOMATION_GOOGLE_CLIENT_ID")!,
        scope: [
            "https://www.googleapis.com/auth/userinfo.email",
            "https://www.googleapis.com/auth/drive.readonly",
        ],
        authorization_url_extras: {
            prompt: "consent",
            access_type: "offline",
        },
        identity_config: {
            url: "https://www.googleapis.com/oauth2/v1/userinfo",
            account_identifier: "$.email",
        },
    },
});

const GitHubProvider = DefineOAuth2Provider({
    provider_key: "github",
    provider_type: Schema.providers.oauth2.CUSTOM,
    options: {
        provider_name: "GitHub",
        authorization_url: "https://github.com/login/oauth/authorize",
        token_url: "https://github.com/login/oauth/access_token",
        client_id: Deno.env.get("SLACK_AUTOMATION_GITHUB_CLIENT_ID")!,
        scope: [
            "repo",
            "read:org",
            "read:user",
            "user:email",
            "read:enterprise",
        ],
        identity_config: {
            url: "https://api.github.com/user",
            account_identifier: "$.login",
        },
    },
});

const SlackProvider = DefineOAuth2Provider({
    provider_key: "my_slack",
    provider_type: Schema.providers.oauth2.CUSTOM,
    options: {
        provider_name: "Slack",
        authorization_url: "https://slack.com/oauth/v2/authorize",
        token_url: "https://slack.com/api/oauth.v2.access",
        client_id: Deno.env.get("SLACK_AUTOMATION_SLACK_CLIENT_ID")!,
        scope: [
            "channels:history",
        ],
        identity_config: {
            url: "https://slack.com/api/users.identity",
            account_identifier: "$.user.id",
        },
    },
});

export default Manifest({
    name: "hippocampus",
    description: "",
    icon: "../images/Kai.jpg",
    functions: [
        InvokeOpenAI,
        OpenGoogleAppsScript,
        OpenGitHubFile,
        SendMessage,
        SendMessageFromCSV,
        ReplyMessageFromCSV,
        RetrieveMessage,
        RetrieveMessagesFromReaction,
    ],
    workflows: [],
    externalAuthProviders: [GoogleProvider, GitHubProvider, SlackProvider],
    outgoingDomains: [
        "api.openai.com",
        "script.google.com",
        "script.googleusercontent.com",
        "github.com",
        "raw.githubusercontent.com",
    ],
    botScopes: ["chat:write", "chat:write.public", "channels:join", "channels:history"],
});
