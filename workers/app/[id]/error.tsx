"use client";

import { AlertCircle } from "lucide-react";
import { useEffect } from "react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";

export default function ErrorPage({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error(error);
  }, [error]);
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
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>
            Failed to load paste. Please try again.
          </AlertDescription>
        </Alert>
        <div className="mt-4">
          <Button onClick={reset}>Try again</Button>
        </div>
      </main>
    </div>
  );
}
