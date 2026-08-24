import { matchesIfNoneMatch } from "@/lib/etag";
import { problem } from "@/lib/problem";
import { getPasteRepository } from "@/lib/storage";

// A paste can expire or be deleted, so never serve this without asking.
// The ETag still turns an unchanged revalidation into a bodyless 304.
const CACHE_CONTROL = "no-cache";

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

    const { etag, ...body } = paste;

    // A 304 has to repeat what the 200 would have sent, so share the headers.
    const headers = { ETag: etag, "Cache-Control": CACHE_CONTROL };

    if (matchesIfNoneMatch(request, etag)) {
      return new Response(null, { status: 304, headers });
    }

    return Response.json(body, { headers });
  } catch (error) {
    console.error("Failed to get paste:", error);
    return problem(500, "Failed to get paste");
  }
}
