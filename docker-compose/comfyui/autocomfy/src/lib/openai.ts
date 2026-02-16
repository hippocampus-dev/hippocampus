import type { PromptText } from "./types";

const OPENAI_BASE_URL = process.env.OPENAI_BASE_URL;
const OPENAI_API_KEY = process.env.OPENAI_API_KEY;
const OPENAI_MODEL = process.env.OPENAI_MODEL || "gpt-4o";

interface TextNode {
  nodeId: string;
  inputName: string;
  currentText: string;
}

export async function generatePromptTexts(
  concept: string,
  textNodes: TextNode[],
  goodExamples: { prompts: PromptText[]; reason: string }[],
): Promise<Record<string, string>> {
  if (!OPENAI_BASE_URL) return {};

  const systemPrompt = [
    "You rewrite text for ComfyUI TextEncode inputs.",
    "Rules:",
    '1) Output JSON only: {"texts":{"nodeId:inputName":"generated text"}}',
    "2) Return one entry for every listed field. Do not omit any.",
    "3) Rewrite every field from scratch to match the concept. Never keep currentText unchanged.",
    "4) No copying from currentText or examples. Do not reuse any phrase from them.",
    "5) If inputName contains 'negative', write a negative prompt. Otherwise write a positive prompt.",
    "6) Use examples only for style patterns (format, detail density, section flow). Pay attention to 'Why it was good' notes.",
    "7) Keep the user concept central in every generated field.",
  ].join("\n");

  const nodeDescriptions = textNodes
    .map(
      (n) =>
        `- nodeId: ${n.nodeId}, inputName: ${n.inputName}, currentText: "${n.currentText}"`,
    )
    .join("\n");

  let userMessage = `Concept: ${concept}\n\nTargets (must all be rewritten):\n${nodeDescriptions}`;

  if (goodExamples.length > 0) {
    const examplesText = goodExamples
      .slice(0, 5)
      .map((example, i) => {
        const lines = example.prompts
          .map((p) => `  ${p.nodeId}:${p.inputName} = "${p.text}"`)
          .join("\n");
        const reasonLine = example.reason
          ? `  Why it was good: ${example.reason}`
          : "";
        return `Example ${i + 1}:\n${lines}${reasonLine ? "\n" + reasonLine : ""}`;
      })
      .join("\n");
    userMessage += `\n\nGood examples (style only, never copy wording):\n${examplesText}`;
  }

  const response = await fetch(`${OPENAI_BASE_URL}/chat/completions`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${OPENAI_API_KEY}`,
    },
    body: JSON.stringify({
      model: OPENAI_MODEL,
      messages: [
        { role: "system", content: systemPrompt },
        { role: "user", content: userMessage },
      ],
      response_format: { type: "json_object" },
    }),
    signal: AbortSignal.timeout(60_000),
  });

  if (!response.ok) {
    const body = await response.text();
    throw new Error(
      `OpenAI API error: status=${response.status}, body=${body}`,
    );
  }

  const data: {
    choices: { message: { content: string } }[];
  } = await response.json();

  const content = data.choices[0]?.message?.content;
  if (!content) return {};

  const parsed: { texts?: Record<string, string> } = JSON.parse(content);
  return parsed.texts ?? {};
}
