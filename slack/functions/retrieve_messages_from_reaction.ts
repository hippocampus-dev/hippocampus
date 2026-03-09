import {DefineFunction, Schema, SlackFunction} from "deno-slack-sdk/mod.ts";
import {SlackAPI} from "deno-slack-api/mod.ts";

export const def = DefineFunction({
    callback_id: "retrieve_messages_from_reaction",
    title: "Retrieve messages from reaction",
    source_file: "functions/retrieve_messages_from_reaction.ts",
    input_parameters: {
        properties: {
            parent_message_link: {
                type: Schema.types.string,
                description: "Link to the parent message",
            },
            reacted_message_link: {
                type: Schema.types.string,
                description: "Link to the message that was reacted to",
            },
        },
        required: ["parent_message_link", "reacted_message_link"],
    },
    output_parameters: {
        properties: {
            parent_message: {
                type: Schema.types.string,
            },
            reacted_message: {
                type: Schema.types.string,
            },
        },
        required: ["parent_message", "reacted_message"],
    },
});

export default SlackFunction(def, async (
    { inputs, token }: {
        inputs: { parent_message_link: string; reacted_message_link: string };
        token: string;
    },
): Promise<
    { outputs: { parent_message: string; reacted_message: string } } | {
        error: string;
    }
> => {
    // e.g. https://deno.slack.com/archives/C01U6P7JZ9W/p1627985207000100
    const { parent_message_link, reacted_message_link } = inputs;

    try {
        const [parent, reacted] = await Promise.all(
            [parent_message_link, reacted_message_link].map(async (link) => {
                const url = new URL(link);
                const [channel_id, timestamp] = url.pathname.split("/").slice(
                    -2,
                );
                if (!channel_id || !timestamp) {
                    throw new Error("Invalid message link");
                }
                const ts = parseFloat(
                    timestamp.slice(1, 11) + "." + timestamp.slice(11),
                );

                const client = SlackAPI(token);

                await client.conversations.join({
                    channel: channel_id,
                });

                return await client.conversations.replies({
                    channel: channel_id,
                    ts: ts,
                    limit: 1,
                });
            }),
        );

        if (parent.error) {
            return {
                error: `Failed to retrieve parent message: ${parent.error}`,
            };
        }
        if (reacted.error) {
            return {
                error: `Failed to retrieve reacted message: ${reacted.error}`,
            };
        }

        return {
            outputs: {
                parent_message: parent.messages[0].text,
                reacted_message: reacted.messages[0].text,
            },
        };
    } catch (e) {
        return {
            error: e.stack,
        };
    }
});
