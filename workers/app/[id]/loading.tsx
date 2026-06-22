import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";

export default function Loading() {
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
              <Skeleton className="h-8 w-48" />
              <div className="flex items-center gap-3 mt-2">
                <Skeleton className="h-5 w-20 rounded-full" />
                <Separator orientation="vertical" className="h-4" />
                <Skeleton className="h-4 w-16" />
                <Separator orientation="vertical" className="h-4" />
                <Skeleton className="h-4 w-32" />
              </div>
            </div>
            <Skeleton className="h-9 w-32 rounded-md" />
          </div>

          <Skeleton className="h-96 w-full rounded-lg" />
        </div>
      </main>
    </div>
  );
}
