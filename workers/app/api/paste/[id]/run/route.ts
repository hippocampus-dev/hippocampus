import { problem } from "@/lib/problem";
import { getCodeRunner } from "@/lib/runner";
import { getPasteRepository } from "@/lib/storage";

const RUNNABLE_LANGUAGES = new Set(["javascript", "python"]);

export async function POST(
  _request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    const repository = await getPasteRepository();

    const paste = await repository.findById(id);
    if (!paste) {
      return problem(404, "Paste not found");
    }

    if (!RUNNABLE_LANGUAGES.has(paste.language)) {
      return problem(
        400,
        `Language "${paste.language}" is not supported for execution`,
      );
    }

    const runner = await getCodeRunner();
    const result = await runner.run(paste.content, paste.language);

    return Response.json(result);
  } catch (error) {
    console.error("Failed to run paste:", error);
    return problem(500, "Failed to run paste");
  }
}
