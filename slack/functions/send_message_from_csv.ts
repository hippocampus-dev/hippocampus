// https://github.com/slackapi/deno-slack-sdk/blob/main/src/schema/slack/functions/send_message.ts

import {DefineFunction, Schema, SlackFunction} from "deno-slack-sdk/mod.ts";
import {SlackAPI} from "deno-slack-api/mod.ts";
import render from "nano/mod.ts";
import {parseRFC4180} from "./parser/csv.ts";

export const def = DefineFunction({
    callback_id: "send_message_from_csv",
    title: "Send a message to a channel from CSV",
    source_file: "functions/send_message_from_csv.ts",
    input_parameters: {
        properties: {
            channel_id: {
                type: Schema.slack.types.channel_id,
                description: "Search all channels",
                title: "Select a channel",
            },
            csv: {
                type: Schema.types.string,
            },
            template: {
                type: Schema.slack.types.rich_text,
                description: "Add a message",
                title: "Add a message",
            },
        },
        required: ["channel_id", "csv", "template"],
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
const destructiveRenderingRecursively = (elements: Element[], data: object): Promise<object> => {
    return Promise.all(elements.map(async (element) => {
        if (element.text) {
            element.text = await render(element.text, data);
        }

        if (element.elements) {
            return await destructiveRenderingRecursively(element.elements, data);
        }
    }));
}

export default SlackFunction(def, async (
    {inputs, token}: { inputs: { channel_id: string, csv: string, template: Element[] }, token: string },
): Promise<{ outputs: { channel_id: string } } | { error: string }> => {
    const {channel_id, csv, template} = inputs;

    try {
        const client = SlackAPI(token);

        const rows = parseRFC4180(csv);
        const header = rows[0];

        const responses = await Promise.all(rows.slice(1).map(async (row) => {
            const data = new Map<string, string>();
            header.forEach((key, i) => {
                data.set(key, row[i]);
            });

            const blocks = JSON.parse(JSON.stringify(template));
            await destructiveRenderingRecursively(blocks, Object.fromEntries(data));

            return client.chat.postMessage({
                channel: channel_id,
                blocks: blocks,
            });
        }));
        for (const response of responses) {
            if (response.error) {
                return {
                    error: `Failed to send message: ${response.error}`,
                };
            }
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
