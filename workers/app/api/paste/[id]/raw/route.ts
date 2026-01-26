import { getPasteRepository } from "@/lib/storage";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ id: string }> }
) {
  try {
    const { id } = await params;
    const repository = await getPasteRepository();

    const paste = await repository.findById(id);
    if (!paste) {
      return new Response("Paste not found", { status: 404 });
    }

    return new Response(paste.content, {
      headers: {
        "Content-Type": "text/plain; charset=utf-8",
        "Cache-Control": "public, max-age=3600",
      },
    });
  } catch (error) {
    console.error("Failed to get raw paste:", error);
    return new Response("Failed to get raw paste", { status: 500 });
  }
}
