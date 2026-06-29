import { AlertCircle } from "lucide-react";
import { Alert, AlertDescription } from "@/components/ui/alert";

export default function NotFound() {
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
        <Alert>
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>Paste not found or has expired.</AlertDescription>
        </Alert>
      </main>
    </div>
  );
}
