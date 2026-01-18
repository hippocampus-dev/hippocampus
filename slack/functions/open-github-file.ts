import {DefineFunction, Schema, SlackFunction} from "deno-slack-sdk/mod.ts";
import {SlackAPI} from "deno-slack-api/mod.ts";

export { DefineConnector } from "deno-slack-sdk/functions/definitions/mod.ts";

export const def = DefineFunction({
    callback_id: "open-github-file",
    title: "Open GitHub file",
    source_file: "functions/open-github-file.ts",
    input_parameters: {
        properties: {
            github_access_token_id: {
                type: Schema.slack.types.oauth2,
                oauth2_provider_key: "github",
            },
            // deno-slack-sdk does not support dynamic inputs yet
            // https://github.com/slackapi/deno-slack-sdk/issues/230
            url: {
                type: Schema.types.string,
            },
        },
        required: ["github_access_token_id", "url"],
    },
    output_parameters: {
        properties: {
            content: {
                type: Schema.types.string,
            },
        },
        required: ["content"],
    },
});

const pattern =
    /^https:\/\/github.com\/([^\/]+)\/([^\/]+)\/blob\/([^\/]+)\/(.+)$/;

export default SlackFunction(def, async (
    { inputs, token }: {
        inputs: { github_access_token_id: string; url: string };
        token: string;
    },
): Promise<{ outputs: { content: string } } | { error: string }> => {
    const { url } = inputs;

    try {
        const client = SlackAPI(token);

        const auth = await client.apps.auth.external.get({
            external_token_id: inputs.github_access_token_id,
        });

        if (auth.error) {
            return {
                error: auth.error,
            };
        }

        const destination = url.replace(
            pattern,
            "https://raw.githubusercontent.com/$1/$2/$3/$4",
        );
        const response = await fetch(destination, {
            method: "GET",
            headers: {
                "Authorization": `Bearer ${auth.external_token}`,
            },
        });
        if (!response.ok) {
            return {
                error: `Failed to fetch: ${response.statusText}`,
            };
        }
        return {
            outputs: {
                content: await response.text(),
            },
        };
    } catch (e) {
        return {
            error: e.stack,
        };
    }
});
