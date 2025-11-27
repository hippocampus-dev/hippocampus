// https://github.com/slackapi/deno-slack-sdk/blob/main/src/schema/slack/functions/reply_in_thread.ts

import {DefineFunction, Schema, SlackFunction} from "deno-slack-sdk/mod.ts";
import {SlackAPI} from "deno-slack-api/mod.ts";
import render from "nano/mod.ts";
import {parseRFC4180} from "./parser/csv.ts";

export const def = DefineFunction({
    callback_id: "reply_in_thread_from_csv",
    title: "Reply to a message in thread from CSV",
    source_file: "functions/reply_in_thread_from_csv.ts",
    input_parameters: {
        properties: {
            message_context: {
                type: Schema.slack.types.message_context,
                description: "Select a message to reply to",
                title: "Select a message to reply to",
            },
            csv: {
                type: Schema.types.string,
            },
            template: {
                type: Schema.slack.types.rich_text,
                description: "Add a reply",
                title: "Add a reply",
            },
        },
        required: ["message_context", "csv", "template"],
    },
    output_parameters: {
        properties: {
            channel_id: {
                type: Schema.slack.types.channel_id,
            },
        },
        required: ["channel_id"],
    },
});

type Element = {
    text?: string;
    elements: Element[];
};
const destructiveRenderingRecursively = (
    elements: Element[],
    data: object,
): Promise<object> => {
    return Promise.all(elements.map(async (element) => {
        if (element.text) {
            element.text = await render(element.text, data);
        }

        if (element.elements) {
            return await destructiveRenderingRecursively(
                element.elements,
                data,
            );
        }
    }));
};

export default SlackFunction(def, async (
    { inputs, token }: {
        inputs: {
            message_context: {
                message_ts: string;
                user_id?: string;
                channel_id?: string;
            };
            csv: string;
            template: Element[];
        };
        token: string;
    },
): Promise<{ outputs: { channel_id: string } } | { error: string }> => {
    const { message_context, csv, template } = inputs;

    try {
        const client = SlackAPI(token);

        const rows = parseRFC4180(csv);
        const header = rows[0];

        const responses = await Promise.all(
            rows.slice(1).map(async (row) => {
                const data = new Map<string, string>();
                header.forEach((key, i) => {
                    data.set(key, row[i]);
                });

                const blocks = JSON.parse(JSON.stringify(template));
                await destructiveRenderingRecursively(
                    blocks,
                    Object.fromEntries(data),
                );

                return client.chat.postMessage({
                    channel: message_context.channel_id!,
                    thread_ts: message_context.message_ts,
                    blocks: blocks,
                });
            }),
        );
        for (const response of responses) {
            if (response.error) {
                return {
                    error: `Failed to send message: ${response.error}`,
                };
            }
        }

        return {
            outputs: {
                channel_id: message_context.channel_id!,
            },
        };
    } catch (e) {
        return {
            error: e.stack,
        };
    }
});
