import {DefineFunction, Schema, SlackFunction} from "deno-slack-sdk/mod.ts";
import {SlackAPI} from "deno-slack-api/mod.ts";

export const def = DefineFunction({
    callback_id: "retrieve_message",
    title: "Retrieve message from a message link",
    source_file: "functions/retrieve_message.ts",
    input_parameters: {
        properties: {
            message_link: {
                type: Schema.types.string
            },
        },
        required: ["message_link"],
    },
    output_parameters: {
        properties: {
            text: {
                type: Schema.types.string
            },
        },
        required: ["text"],
    },
});

export default SlackFunction(def, async (
    {inputs, token}: { inputs: { message_link: string }, token: string },
): Promise<{ outputs: { text: string } } | { error: string }> => {
    // e.g. https://deno.slack.com/archives/C01U6P7JZ9W/p1627985207000100
    const {message_link} = inputs;

    try {
        const url = new URL(message_link);
        const [channel_id, timestamp] = url.pathname.split("/").slice(-2);
        if (!channel_id || !timestamp) {
            return {
                error: "Invalid message link",
            };
        }
        const ts = parseFloat(timestamp.slice(1, 11) + "." + timestamp.slice(11));

        const client = SlackAPI(token);

        await client.conversations.join({
            channel: channel_id,
        });

        const result = await client.conversations.replies({
            channel: channel_id,
            ts: ts,
            limit: 1,
        });

        if (result.error) {
            return {
                error: `Failed to retrieve message: ${result.error}`,
            };
        }

        return {
            outputs: {
                text: result.messages[0].text,
            },
        };
    } catch (e) {
        return {
            error: e.stack,
        };
    }
});
