import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import type { ComfyUIOutputFile, LabelData } from "@/lib/types";

interface ResultCardProps {
  file: ComfyUIOutputFile;
  label?: LabelData;
  onClick: () => void;
}

export default function ResultCard({ file, label, onClick }: ResultCardProps) {
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

  return (
    <Card
      className="overflow-hidden cursor-pointer hover:ring-2 hover:ring-primary transition-all"
      onClick={onClick}
    >
      <div className="relative aspect-square bg-muted">
        {isVideo ? (
          <video
            src={src}
            className="w-full h-full object-cover"
            muted
            loop
            playsInline
            onMouseEnter={(e) => e.currentTarget.play()}
            onMouseLeave={(e) => {
              e.currentTarget.pause();
              e.currentTarget.currentTime = 0;
            }}
          />
        ) : isAudio ? (
          <div className="w-full h-full flex flex-col items-center justify-center gap-4 p-4">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="w-12 h-12 text-muted-foreground"
            >
              <path d="M9 18V5l12-2v13" />
              <circle cx="6" cy="18" r="3" />
              <circle cx="18" cy="16" r="3" />
            </svg>
            <audio
              src={src}
              controls
              className="w-full max-w-[200px]"
              onClick={(e) => e.stopPropagation()}
            />
          </div>
        ) : (
          <img
            src={src}
            alt={file.filename}
            className="w-full h-full object-cover"
            loading="lazy"
          />
        )}

        {label && (
          <Badge
            variant={label.label === "good" ? "default" : "destructive"}
            className="absolute top-2 right-2"
          >
            {label.label}
          </Badge>
        )}
      </div>
      <div className="p-2">
        <p className="text-xs text-muted-foreground truncate">
          {file.filename}
        </p>
      </div>
    </Card>
  );
}
