import { problem } from "@/lib/problem";
import { getPasteRepository } from "@/lib/storage";

const MAX_CONTENT_SIZE = 1024 * 1024; // 1MB

function parseExpiry(expiry: string): Date | null {
  if (!expiry) return null;

  const now = new Date();
  const match = expiry.match(/^(\d+)([hdm])$/);
  if (!match) return null;

  const [, amount, unit] = match;
  const num = parseInt(amount, 10);

  switch (unit) {
    case "h":
      return new Date(now.getTime() + num * 60 * 60 * 1000);
    case "d":
      return new Date(now.getTime() + num * 24 * 60 * 60 * 1000);
    case "m":
      return new Date(now.getTime() + num * 30 * 24 * 60 * 60 * 1000);
    default:
      return null;
  }
}

export async function POST(request: Request) {
  try {
    const repository = await getPasteRepository();

    const body = await request.json();
    const { content, language, title, expiry } = body as {
      content: string;
      language: string;
      title: string;
      expiry: string;
    };

    if (!content || typeof content !== "string") {
      return problem(400, "Content is required");
    }

    if (content.length > MAX_CONTENT_SIZE) {
      return problem(413, "Content too large");
    }

    const metadata = await repository.create({
      content,
      language: language || "text",
      title: title || "Untitled",
      expiresAt: parseExpiry(expiry),
    });

    return Response.json(metadata, {
      status: 201,
      headers: { Location: `/api/paste/${metadata.id}` },
    });
  } catch (error) {
    console.error("Failed to create paste:", error);
    return problem(500, "Failed to create paste");
  }
}
