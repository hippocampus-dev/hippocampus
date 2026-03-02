import { createFileRoute } from "@tanstack/react-router";
import ResultGallery from "@/components/ResultGallery";

export const Route = createFileRoute("/results")({ component: ResultsPage });

function ResultsPage() {
  return (
    <div className="min-h-screen bg-background">
      <main className="max-w-6xl mx-auto px-4 py-8 space-y-6">
        <h1 className="text-2xl font-bold">Results</h1>
        <ResultGallery />
      </main>
    </div>
  );
}
