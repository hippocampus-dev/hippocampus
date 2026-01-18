"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import { AlertCircle, Sparkles } from "lucide-react";

export default function AiExplain({
  id,
  language,
}: {
  id: string;
  language: string;
}) {
  const [explanation, setExplanation] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleExplain = async () => {
    setIsLoading(true);
    setError(null);

    try {
      const response = await fetch(`/api/paste/${id}/explain`, {
        method: "POST",
      });

      if (!response.ok) {
        const errorData = (await response.json()) as { error?: string };
        throw new Error(errorData.error || "Failed to generate explanation");
      }

      const { explanation } = (await response.json()) as { explanation: string };
      setExplanation(explanation);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setIsLoading(false);
    }
  };

  if (language === "text") {
    return null;
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <div>
          <CardTitle className="text-lg">AI Explanation</CardTitle>
          <CardDescription>
            Get an AI-generated explanation of this code
          </CardDescription>
        </div>
        {!explanation && !isLoading && (
          <Button onClick={handleExplain} disabled={isLoading}>
            <Sparkles className="h-4 w-4" />
            Explain this code
          </Button>
        )}
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
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-3/4" />
          </div>
        )}

        {explanation && (
          <div className="whitespace-pre-wrap text-muted-foreground">
            {explanation}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
