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
      return Response.json({ error: "Paste not found" }, { status: 404 });
    }

    if (!RUNNABLE_LANGUAGES.has(paste.language)) {
      return Response.json(
        { error: `Language "${paste.language}" is not supported for execution` },
        { status: 400 },
      );
    }

    const runner = await getCodeRunner();
    const result = await runner.run(paste.content, paste.language);

    return Response.json(result);
  } catch (error) {
    console.error("Failed to run paste:", error);
    return Response.json({ error: "Failed to run paste" }, { status: 500 });
  }
}
