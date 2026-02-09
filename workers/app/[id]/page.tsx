import { notFound } from "next/navigation";
import { createHighlighter, type Highlighter } from "shiki";
import { createJavaScriptRegexEngine } from "shiki/engine/javascript";
import { getPasteRepository } from "@/lib/storage";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import PasteActions from "@/components/PasteActions";
import AiExplain from "@/components/AiExplain";
import CodeBlock from "@/components/CodeBlock";

let highlighter: Highlighter | null = null;

async function getHighlighter() {
  if (!highlighter) {
    highlighter = await createHighlighter({
      themes: ["github-dark"],
      langs: ["javascript", "typescript", "python", "go", "rust", "java", "c", "cpp", "csharp", "ruby", "php", "swift", "kotlin", "scala", "html", "css", "json", "yaml", "toml", "markdown", "sql", "bash", "shell", "plaintext"],
      engine: createJavaScriptRegexEngine(),
    });
  }
  return highlighter;
}

async function getPaste(id: string) {
  const repository = await getPasteRepository();

  const paste = await repository.findById(id);
  if (!paste) {
    return null;
  }

  if (paste.expiresAt && new Date(paste.expiresAt) < new Date()) {
    await repository.deleteById(id);
    return null;
  }

  return paste;
}

function formatDate(dateStr: string) {
  return new Date(dateStr).toLocaleString("en-US", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatSize(bytes: number) {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export default async function PastePage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const paste = await getPaste(id);

  if (!paste) {
    notFound();
  }

  const shiki = await getHighlighter();
  const lang = paste.language === "text" ? "plaintext" : paste.language;
  const html = shiki.codeToHtml(paste.content, {
    lang: shiki.getLoadedLanguages().includes(lang) ? lang : "plaintext",
    theme: "github-dark",
    transformers: [
      {
        line(node, line) {
          node.properties["data-line"] = line;
          node.properties["id"] = `L${line}`;

          const lineNumber = {
            type: "element" as const,
            tagName: "span",
            properties: {
              class: "line-number",
              "data-line": line,
            },
            children: [{ type: "text" as const, value: String(line) }],
          };

          node.children.unshift(lineNumber);
        },
      },
    ],
  });

  return (
    <div className="min-h-screen bg-background">
      <header className="border-b">
        <div className="max-w-5xl mx-auto px-4 py-4">
          <a href="/" className="text-xl font-bold font-mono hover:opacity-80">
            Paste
          </a>
        </div>
      </header>

      <main className="max-w-5xl mx-auto px-4 py-8">
        <div className="space-y-6">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold">{paste.title}</h1>
              <div className="flex flex-wrap items-center gap-3 mt-2 text-sm text-muted-foreground">
                <Badge variant="secondary">{paste.language}</Badge>
                <Separator orientation="vertical" className="h-4" />
                <span>{formatSize(paste.size)}</span>
                <Separator orientation="vertical" className="h-4" />
                <span>Created {formatDate(paste.createdAt)}</span>
                {paste.expiresAt && (
                  <>
                    <Separator orientation="vertical" className="h-4" />
                    <span className="text-amber-500">
                      Expires {formatDate(paste.expiresAt)}
                    </span>
                  </>
                )}
              </div>
            </div>
            <PasteActions id={id} content={paste.content} />
          </div>

          <CodeBlock html={html} />

          <AiExplain id={id} language={paste.language} />
        </div>
      </main>
    </div>
  );
}
