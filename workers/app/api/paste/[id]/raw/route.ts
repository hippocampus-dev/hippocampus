import { matchesIfNoneMatch } from "@/lib/etag";
import { problem } from "@/lib/problem";
import { getPasteRepository } from "@/lib/storage";

const CACHE_CONTROL = "public, max-age=3600";

export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  try {
    const { id } = await params;
    const repository = await getPasteRepository();

    const paste = await repository.findById(id);
    if (!paste) {
      return problem(404, "Paste not found");
    }

    // A 304 has to repeat what the 200 would have sent, so share the headers.
    const headers = { ETag: paste.etag, "Cache-Control": CACHE_CONTROL };

    if (matchesIfNoneMatch(request, paste.etag)) {
      return new Response(null, { status: 304, headers });
    }

    return new Response(paste.content, {
      headers: { ...headers, "Content-Type": "text/plain; charset=utf-8" },
    });
  } catch (error) {
    console.error("Failed to get raw paste:", error);
    return problem(500, "Failed to get raw paste");
  }
}
