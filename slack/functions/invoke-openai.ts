import {join} from "std/path/mod.ts";
import {DefineFunction, Schema, SlackFunction} from "deno-slack-sdk/mod.ts";

export const def = DefineFunction({
    callback_id: "invoke-openai",
    title: "Invoke OpenAI",
    source_file: "functions/invoke-openai.ts",
    input_parameters: {
        properties: {
            prompt: {
                type: Schema.types.string,
            },
        },
        required: ["prompt"],
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

const getOpenAIAPIBase = () => {
    const base = Deno.env.get("OPENAI_BASE_URL");
    if (base) {
        return base;
    }
    return "https://api.openai.com/v1";
};

export default SlackFunction(def, async (
    { inputs }: { inputs: { prompt: string } },
): Promise<{ outputs: { content: string } } | { error: string }> => {
    const { prompt } = inputs;

    try {
        const destination = join(getOpenAIAPIBase(), "/chat/completions");
        const response = await fetch(destination, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify({
                messages: [
                    {
                        "role": "user",
                        "content": prompt,
                    },
                ],
            }),
        });

        const json: {
            choices: { message: { role: string; content: string } }[];
        } = await response.json();
        return {
            outputs: {
                content: json.choices[0].message.content,
            },
        };
    } catch (e) {
        return {
            error: e.stack,
        };
    }
});
