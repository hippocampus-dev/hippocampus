// https://github.com/slackapi/deno-slack-sdk/blob/main/src/schema/slack/functions/send_message.ts

import {DefineFunction, Schema, SlackFunction} from "deno-slack-sdk/mod.ts";
import {SlackAPI} from "deno-slack-api/mod.ts";

export const def = DefineFunction({
    callback_id: "send_message",
    title: "Send a message to a channel",
    source_file: "functions/send_message.ts",
    input_parameters: {
        properties: {
            channel_id: {
                type: Schema.slack.types.channel_id,
                description: "Search all channels",
                title: "Select a channel",
            },
            template: {
                type: Schema.slack.types.rich_text,
                description: "Add a message",
                title: "Add a message",
            },
        },
        required: ["channel_id", "template"],
    },
    output_parameters: {
        properties: {
            channel_id: {
                type: Schema.slack.types.channel_id,
            }
        },
        required: ["channel_id"],
    },
});

type Element = {
    text?: string;
    elements: Element[];
}

export default SlackFunction(def, async (
    {inputs, token}: { inputs: { channel_id: string, template: Element[] }, token: string },
): Promise<{ outputs: { channel_id: string } } | { error: string }> => {
    const {channel_id, template} = inputs;

    try {
        const client = SlackAPI(token);

        const blocks = JSON.parse(JSON.stringify(template));

        const response = await client.chat.postMessage({
            channel: channel_id,
            blocks: blocks,
        });
        if (response.error) {
            return {
                error: `Failed to send message: ${response.error}`,
            };
        }

        return {
            outputs: {
                channel_id: channel_id,
            },
        };
    } catch (e) {
        return {
            error: e.stack,
        };
    }
});
