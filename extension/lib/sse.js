export async function* parseSSEStream(reader, abortController) {
    const decoder = new TextDecoder();
    while (true) {
        if (abortController?.signal.aborted) {
            break;
        }

        const {done, value} = await reader.read();
        if (done) {
            break;
        }

        const lines = decoder.decode(value).split("\n").filter((line) => line.startsWith("data: ")).map((line) => line.slice(6).trim());
        for (const line of lines) {
            if (line === "[DONE]" || line.startsWith(": ping - ")) {
                return;
            }

            const json = JSON.parse(line);
            if (json.error !== undefined) {
                yield {type: "error", message: json.error.message};
                return;
            }
            if (json.choices?.length !== 1) {
                continue;
            }

            const choice = json.choices[0];

            switch (choice.finish_reason) {
                case "length":
                    yield {type: "finish", reason: "length"};
                    return;
                case "content_filter":
                    yield {type: "finish", reason: "content_filter"};
                    return;
                default:
                    break;
            }

            if (choice.delta.content === null) {
                continue;
            }

            yield {type: "content", content: choice.delta.content};
        }
    }
}
