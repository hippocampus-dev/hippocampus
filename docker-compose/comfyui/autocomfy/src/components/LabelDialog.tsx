import { useState, useEffect } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { saveLabel } from "@/server/functions";
import type { ComfyUIOutputFile, LabelData } from "@/lib/types";

interface LabelDialogProps {
  file: ComfyUIOutputFile | null;
  existingLabel?: LabelData;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSave: (label: LabelData) => void;
}

export default function LabelDialog({
  file,
  existingLabel,
  open,
  onOpenChange,
  onSave,
}: LabelDialogProps) {
  const [label, setLabel] = useState<"good" | "bad">(
    existingLabel?.label ?? "good",
  );
  const [reason, setReason] = useState(existingLabel?.reason ?? "");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setLabel(existingLabel?.label ?? "good");
    setReason(existingLabel?.reason ?? "");
  }, [existingLabel, file]);

  if (file === null) return null;

  const isVideo =
    file.filename.endsWith(".mp4") ||
    file.filename.endsWith(".webm") ||
    file.filename.endsWith(".gif");

  const isAudio =
    file.filename.endsWith(".mp3") ||
    file.filename.endsWith(".wav") ||
    file.filename.endsWith(".flac") ||
    file.filename.endsWith(".ogg");

  const src = `/api/results/${encodeURIComponent(file.filename)}?${new URLSearchParams({
    ...(file.subfolder && { subfolder: file.subfolder }),
    ...(file.type && { type: file.type }),
  }).toString()}`;

  const handleSave = async () => {
    setSaving(true);
    try {
      const data = await saveLabel({
        data: { filename: file.filename, label, reason },
      });
      onSave(data);
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle className="truncate">{file.filename}</DialogTitle>
        </DialogHeader>

        <div className="bg-muted rounded-lg overflow-hidden">
          {isVideo ? (
            <video
              src={src}
              className="w-full max-h-96 object-contain"
              controls
              autoPlay
              loop
              muted
            />
          ) : isAudio ? (
            <div className="flex items-center justify-center p-8">
              <audio src={src} controls className="w-full" autoPlay />
            </div>
          ) : (
            <img
              src={src}
              alt={file.filename}
              className="w-full max-h-96 object-contain"
            />
          )}
        </div>

        {file.prompts && file.prompts.length > 0 && (
          <div className="space-y-2">
            <Label>Prompts</Label>
            <div className="space-y-2 max-h-80 overflow-y-auto">
              {file.prompts.map((prompt) => (
                <div
                  key={`${prompt.nodeId}-${prompt.inputName}`}
                  className="rounded-md bg-muted p-3 text-sm"
                >
                  <p className="text-xs text-muted-foreground mb-1">
                    {prompt.inputName} — {prompt.nodeType} (#{prompt.nodeId})
                  </p>
                  <p className="whitespace-pre-wrap break-words">
                    {prompt.text}
                  </p>
                </div>
              ))}
            </div>
          </div>
        )}

        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Label</Label>
            <RadioGroup
              value={label}
              onValueChange={(value) => {
                if (value === "good" || value === "bad") {
                  setLabel(value);
                }
              }}
              className="flex gap-4"
            >
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="good" id="label-good" />
                <Label htmlFor="label-good">Good</Label>
              </div>
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="bad" id="label-bad" />
                <Label htmlFor="label-bad">Bad</Label>
              </div>
            </RadioGroup>
          </div>

          <div className="space-y-2">
            <Label htmlFor="reason">Reason</Label>
            <Textarea
              id="reason"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="Describe why this result is good or bad..."
              rows={3}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSave} disabled={saving}>
            {saving ? "Saving..." : "Save"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
