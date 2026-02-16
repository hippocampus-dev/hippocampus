import { getAiProvider } from "@/lib/ai";
import { getPasteRepository, getExplanationRepository } from "@/lib/storage";

const DEFAULT_CACHE_TTL = 3600;

export async function POST(
  _request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const repository = await getPasteRepository();
    const explanationRepository = await getExplanationRepository();
    const ai = await getAiProvider();

    const cachedExplanation = await explanationRepository.get(id);
    if (cachedExplanation) {
      return Response.json({ explanation: cachedExplanation });
    }

    const paste = await repository.findById(id);
    if (!paste) {
      return Response.json({ error: "Paste not found" }, { status: 404 });
    }

    const truncatedContent = paste.content.slice(0, 4000);

    const response = await ai.createChatCompletion({
      messages: [
        {
          role: "system",
          content: `You are a helpful programming assistant. Explain the following ${paste.language} code concisely. Focus on:
1. What the code does
2. Key concepts or patterns used
3. Any notable implementation details

Keep your explanation clear and beginner-friendly.`,
        },
        {
          role: "user",
          content: truncatedContent,
        },
      ],
      maxTokens: 1024,
    });

    const explanation = response.content;

    let cacheTtl: number | undefined = DEFAULT_CACHE_TTL;
    if (paste.expiresAt) {
      const ttl = Math.floor(
        (new Date(paste.expiresAt).getTime() - Date.now()) / 1000
      );
      cacheTtl = ttl >= 60 ? ttl : undefined;
    }

    if (cacheTtl !== undefined) {
      await explanationRepository.set(id, explanation, cacheTtl);
    }

    return Response.json({ explanation });
  } catch (error) {
    console.error("Failed to explain paste:", error);
    return Response.json(
      { error: "Failed to explain paste" },
      { status: 500 }
    );
  }
}
