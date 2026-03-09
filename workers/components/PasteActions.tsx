"use client";

import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { Copy, Download } from "lucide-react";
import { toast } from "sonner";

export default function PasteActions({
  id,
  content,
}: {
  id: string;
  content: string;
}) {
  const handleCopyCode = async () => {
    try {
      await navigator.clipboard.writeText(content);
      toast.success("Code copied to clipboard");
    } catch {
      toast.error("Failed to copy code to clipboard");
    }
  };

  const handleRaw = () => {
    window.open(`/api/paste/${id}/raw`, "_blank");
  };

  return (
    <TooltipProvider>
      <div className="flex items-center gap-1 p-1 bg-card border rounded-md">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="sm" onClick={handleCopyCode}>
              <Copy className="h-4 w-4" />
              <span>Copy</span>
            </Button>
          </TooltipTrigger>
          <TooltipContent>Copy code to clipboard</TooltipContent>
        </Tooltip>

        <Separator orientation="vertical" className="h-4" />

        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="ghost" size="sm" onClick={handleRaw}>
              <Download className="h-4 w-4" />
              <span>Raw</span>
            </Button>
          </TooltipTrigger>
          <TooltipContent>View raw content</TooltipContent>
        </Tooltip>
      </div>
    </TooltipProvider>
  );
}
