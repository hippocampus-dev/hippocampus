"use client";

import { AlertCircle, Play, Terminal } from "lucide-react";
import { useState } from "react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

interface RunOutput {
  level: "log" | "warn" | "error" | "exception";
  message: string;
}

interface RunResult {
  output: RunOutput[];
  error: string | null;
  wallTimeMs: number;
  timedOut: boolean;
}

const RUNNABLE_LANGUAGES = new Set(["javascript", "python"]);

const OUTPUT_COLORS: Record<RunOutput["level"], string> = {
  log: "text-foreground",
  warn: "text-yellow-400",
  error: "text-red-400",
  exception: "text-red-400",
};

export default function CodeRunner({
  id,
  language,
}: {
  id: string;
  language: string;
}) {
  const [result, setResult] = useState<RunResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleRun = async () => {
    setIsLoading(true);
    setError(null);
    setResult(null);

    try {
      const response = await fetch(`/api/paste/${id}/run`, {
        method: "POST",
      });

      if (!response.ok) {
        const errorData = (await response.json()) as { error?: string };
        throw new Error(errorData.error || "Failed to run code");
      }

      const data = (await response.json()) as RunResult;
      setResult(data);
    } catch (err) {
      console.error("Failed to run code:", err);
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setIsLoading(false);
    }
  };

  if (!RUNNABLE_LANGUAGES.has(language)) {
    return null;
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <div>
          <CardTitle className="text-lg">Run Code</CardTitle>
          <CardDescription>
            Execute this code in a sandboxed environment
          </CardDescription>
        </div>
        <Button onClick={handleRun} disabled={isLoading}>
          <Play className="h-4 w-4" />
          {isLoading ? "Running..." : "Run"}
        </Button>
      </CardHeader>
      <CardContent>
        {error && (
          <Alert variant="destructive">
            <AlertCircle className="h-4 w-4" />
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        {isLoading && (
          <div className="space-y-2">
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-3/4" />
          </div>
        )}

        {result && (
          <div className="rounded-md bg-zinc-950 p-4 font-mono text-sm">
            <div className="flex items-center gap-2 text-muted-foreground mb-2">
              <Terminal className="h-3 w-3" />
              <span>
                {result.timedOut ? "Timed out" : "Completed"} in{" "}
                {result.wallTimeMs}ms
              </span>
            </div>

            {result.output.length > 0 ? (
              <div className="space-y-0.5">
                {result.output.map((line, index) => (
                  <div
                    key={`${index}-${line.level}`}
                    className={OUTPUT_COLORS[line.level]}
                  >
                    {line.level === "exception" && (
                      <span className="text-red-600">⚠ </span>
                    )}
                    {line.message}
                  </div>
                ))}
              </div>
            ) : (
              !result.error && (
                <div className="text-muted-foreground italic">
                  No output produced
                </div>
              )
            )}

            {result.error && (
              <div className="mt-2 text-red-400">{result.error}</div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
