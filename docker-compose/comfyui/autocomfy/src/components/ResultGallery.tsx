import { useState, useEffect, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { ChevronLeft, ChevronRight } from "lucide-react";
import ResultCard from "./ResultCard";
import LabelDialog from "./LabelDialog";
import { fetchResults, fetchLabels } from "@/server/functions";
import type { ComfyUIOutputFile, LabelData } from "@/lib/types";

type Filter = "all" | "unlabeled" | "good" | "bad";

const PAGE_SIZE = 24;

export default function ResultGallery({
  refreshKey,
  topologyHash,
}: { refreshKey?: number; topologyHash?: string } = {}) {
  const [files, setFiles] = useState<ComfyUIOutputFile[]>([]);
  const [labels, setLabels] = useState<Map<string, LabelData>>(new Map());
  const [filter, setFilter] = useState<Filter>("all");
  const [page, setPage] = useState(0);
  const [selectedFile, setSelectedFile] = useState<ComfyUIOutputFile | null>(
    null,
  );
  const [dialogOpen, setDialogOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchData = useCallback(async () => {
    try {
      setError(null);
      const [filesData, labelsData] = await Promise.all([
        fetchResults({ data: { topologyHash } }),
        fetchLabels(),
      ]);

      setFiles(filesData.reverse());
      setLabels(new Map(labelsData.map((l) => [l.filename, l])));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load results");
    }
  }, [topologyHash]);

  useEffect(() => {
    fetchData();
  }, [fetchData, refreshKey]);

  useEffect(() => {
    setPage(0);
  }, [topologyHash]);

  const handleFilterChange = (value: Filter) => {
    setFilter(value);
    setPage(0);
  };

  const handleCardClick = (file: ComfyUIOutputFile) => {
    setSelectedFile(file);
    setDialogOpen(true);
  };

  const handleLabelSave = (label: LabelData) => {
    setLabels((prev) => new Map(prev).set(label.filename, label));
  };

  const filteredFiles = files.filter((file) => {
    const label = labels.get(file.filename);
    switch (filter) {
      case "unlabeled":
        return !label;
      case "good":
        return label?.label === "good";
      case "bad":
        return label?.label === "bad";
      default:
        return true;
    }
  });

  const totalPages = Math.max(1, Math.ceil(filteredFiles.length / PAGE_SIZE));
  const currentPage = Math.min(page, totalPages - 1);
  const pagedFiles = filteredFiles.slice(
    currentPage * PAGE_SIZE,
    (currentPage + 1) * PAGE_SIZE,
  );

  const filters: { value: Filter; label: string }[] = [
    { value: "all", label: `All (${files.length})` },
    {
      value: "unlabeled",
      label: `Unlabeled (${files.filter((f) => !labels.has(f.filename)).length})`,
    },
    {
      value: "good",
      label: `Good (${files.filter((f) => labels.get(f.filename)?.label === "good").length})`,
    },
    {
      value: "bad",
      label: `Bad (${files.filter((f) => labels.get(f.filename)?.label === "bad").length})`,
    },
  ];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex gap-2">
          {filters.map((f) => (
            <Button
              key={f.value}
              variant={filter === f.value ? "default" : "outline"}
              size="sm"
              onClick={() => handleFilterChange(f.value)}
            >
              {f.label}
            </Button>
          ))}
        </div>
        <Button variant="outline" size="sm" onClick={fetchData}>
          Refresh
        </Button>
      </div>

      {error ? (
        <div className="text-center py-12 space-y-2">
          <p className="text-muted-foreground">Failed to load results</p>
          <p className="text-sm text-destructive">{error}</p>
        </div>
      ) : filteredFiles.length === 0 ? (
        <div className="text-center py-12 text-muted-foreground">
          No results found
        </div>
      ) : (
        <>
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-4">
            {pagedFiles.map((file) => (
              <ResultCard
                key={file.filename}
                file={file}
                label={labels.get(file.filename)}
                onClick={() => handleCardClick(file)}
              />
            ))}
          </div>

          {totalPages > 1 && (
            <div className="flex items-center justify-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p) => Math.max(0, p - 1))}
                disabled={currentPage === 0}
              >
                <ChevronLeft className="h-4 w-4" />
              </Button>
              <span className="text-sm text-muted-foreground px-2">
                {currentPage + 1} / {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
                disabled={currentPage >= totalPages - 1}
              >
                <ChevronRight className="h-4 w-4" />
              </Button>
            </div>
          )}
        </>
      )}

      <LabelDialog
        file={selectedFile}
        existingLabel={
          selectedFile ? labels.get(selectedFile.filename) : undefined
        }
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onSave={handleLabelSave}
      />
    </div>
  );
}
