import { createFileRoute } from "@tanstack/react-router";
import { z } from "zod";
import type { Filter } from "@/components/ResultGallery";
import ResultGallery from "@/components/ResultGallery";

const resultsSearchSchema = z.object({
  filter: z.enum(["all", "unlabeled", "good", "bad"]).default("all"),
  page: z.number().nonnegative().int().default(0),
});

export const Route = createFileRoute("/results")({
  validateSearch: resultsSearchSchema,
  component: ResultsPage,
});

function ResultsPage() {
  const { filter, page } = Route.useSearch();
  const navigate = Route.useNavigate();

  return (
    <div className="min-h-screen bg-background">
      <main className="max-w-6xl mx-auto px-4 py-8 space-y-6">
        <h1 className="text-2xl font-bold">Results</h1>
        <ResultGallery
          filter={filter}
          page={page}
          onFilterChange={(value: Filter) =>
            navigate({
              search: { filter: value, page: 0 },
            })
          }
          onPageChange={(value: number) =>
            navigate({
              search: (prev) => ({ ...prev, page: value }),
            })
          }
        />
      </main>
    </div>
  );
}
