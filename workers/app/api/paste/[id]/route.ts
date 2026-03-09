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
      return Response.json({ error: "Paste not found" }, { status: 404 });
    }

    return Response.json(paste);
  } catch (error) {
    console.error("Failed to get paste:", error);
    return Response.json(
      { error: "Failed to get paste" },
      { status: 500 }
    );
  }
}
