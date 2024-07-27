import {DefineFunction, Schema, SlackFunction} from "deno-slack-sdk/mod.ts";
import {SlackAPI} from "deno-slack-api/mod.ts";

export {DefineConnector} from "deno-slack-sdk/functions/definitions/mod.ts";

export const def = DefineFunction({
    callback_id: "open-google-apps-script",
    title: "Open Google Apps Script",
    source_file: "functions/open-google-apps-script.ts",
    input_parameters: {
        properties: {
            google_access_token_id: {
                type: Schema.slack.types.oauth2,
                oauth2_provider_key: "google",
            },
            url: {
                type: Schema.types.string,
            },
        },
        required: ["google_access_token_id", "url"],
    },
    output_parameters: {
        properties: {
            content: {
                type: Schema.types.string,
            }
        },
        required: ["content"],
    },
});

export default SlackFunction(def, async (
    {inputs, token}: { inputs: { google_access_token_id: string, url: string }, token: string }
): Promise<{ outputs: { content: string } } | { error: string }> => {
    const {url} = inputs;

    try {
        const client = SlackAPI(token);

        const auth = await client.apps.auth.external.get({
            external_token_id: inputs.google_access_token_id,
        });

        if (auth.error) {
            return {
                error: auth.error,
            };
        }

        const response = await fetch(url, {
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
