import {
  Flag,
  Pause,
  Play,
  RotateCcw,
  Square,
  Volume2,
  VolumeX,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import type { HandSelection, PlaybackStatus } from "@/lib/types";

interface PlaybackControlsProps {
  status: PlaybackStatus;
  currentTimeMs: number;
  totalDurationMs: number;
  tempoMultiplier: number;
  handSelection: HandSelection;
  volume: number;
  muted: boolean;
  checkpointMs: number | null;
  onPlay: () => void;
  onPause: () => void;
  onStop: () => void;
  onSeek: (timeMs: number) => void;
  onTempoChange: (multiplier: number) => void;
  onHandChange: (hand: HandSelection) => void;
  onVolumeChange: (volume: number) => void;
  onMuteToggle: (muted: boolean) => void;
  onSetCheckpoint: () => void;
  onClearCheckpoint: () => void;
  onPlayFromCheckpoint: () => void;
}

function formatTime(ms: number): string {
  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
}

const TEMPO_OPTIONS = [
  { value: "0.25", label: "0.25x" },
  { value: "0.5", label: "0.5x" },
  { value: "0.75", label: "0.75x" },
  { value: "1", label: "1x" },
  { value: "1.25", label: "1.25x" },
  { value: "1.5", label: "1.5x" },
  { value: "2", label: "2x" },
];

export default function PlaybackControls({
  status,
  currentTimeMs,
  totalDurationMs,
  tempoMultiplier,
  handSelection,
  volume,
  muted,
  checkpointMs,
  onPlay,
  onPause,
  onStop,
  onSeek,
  onTempoChange,
  onHandChange,
  onVolumeChange,
  onMuteToggle,
  onSetCheckpoint,
  onClearCheckpoint,
  onPlayFromCheckpoint,
}: PlaybackControlsProps) {
  const checkpointProgress =
    checkpointMs !== null && totalDurationMs > 0
      ? (checkpointMs / totalDurationMs) * 100
      : null;

  return (
    <div className="flex flex-col gap-3 p-3 bg-card rounded-lg border border-border">
      <div className="flex items-center gap-2">
        <div className="relative flex-1">
          <Slider
            aria-label="Seek"
            min={0}
            max={totalDurationMs}
            step={100}
            value={[currentTimeMs]}
            onValueChange={([value]) => onSeek(value)}
          />
          {checkpointProgress !== null && (
            <div
              className="absolute top-1/2 -translate-y-1/2 w-1 h-3 bg-amber-500 rounded-sm pointer-events-none"
              style={{ left: `${checkpointProgress}%` }}
            />
          )}
        </div>
        <span className="text-xs text-muted-foreground tabular-nums min-w-16 text-right">
          {formatTime(currentTimeMs)} / {formatTime(totalDurationMs)}
        </span>
      </div>

      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          {status === "playing" ? (
            <Button size="icon-lg" onClick={onPause} aria-label="Pause">
              <Pause className="size-5" />
            </Button>
          ) : (
            <Button size="icon-lg" onClick={onPlay} aria-label="Play">
              <Play className="size-5" />
            </Button>
          )}
          <Button
            variant="secondary"
            size="icon"
            onClick={onStop}
            aria-label="Stop"
          >
            <Square className="size-4" />
          </Button>
        </div>

        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={onSetCheckpoint}
            aria-label="Set checkpoint"
            className={checkpointMs !== null ? "text-amber-500" : ""}
          >
            <Flag className="size-4" />
          </Button>
          {checkpointMs !== null && (
            <>
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={onPlayFromCheckpoint}
                aria-label={`Play from ${formatTime(checkpointMs)}`}
                className="text-amber-500 hover:text-amber-400"
              >
                <RotateCcw className="size-4" />
              </Button>
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={onClearCheckpoint}
                aria-label="Clear checkpoint"
              >
                <X className="size-4" />
              </Button>
            </>
          )}
        </div>

        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => onMuteToggle(!muted)}
            aria-label={muted ? "Unmute" : "Mute"}
          >
            {muted ? (
              <VolumeX className="size-4" />
            ) : (
              <Volume2 className="size-4" />
            )}
          </Button>
          <Slider
            aria-label="Volume"
            className="w-20"
            min={0}
            max={1}
            step={0.01}
            value={[muted ? 0 : volume]}
            onValueChange={([value]) => {
              onVolumeChange(value);
              if (value > 0 && muted) onMuteToggle(false);
            }}
          />
        </div>
      </div>

      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">Tempo</span>
          <Select
            value={String(tempoMultiplier)}
            onValueChange={(v) => onTempoChange(Number.parseFloat(v))}
          >
            <SelectTrigger size="sm">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {TEMPO_OPTIONS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex items-center gap-1">
          <span className="text-xs text-muted-foreground mr-1">Hand</span>
          <Button
            variant={handSelection === "left" ? "default" : "outline"}
            size="sm"
            onClick={() => onHandChange("left")}
            aria-label="Left hand"
            className={
              handSelection === "left"
                ? "bg-left-hand hover:bg-left-hand-dark border-left-hand rounded-r-none"
                : "rounded-r-none"
            }
          >
            L
          </Button>
          <Button
            variant={handSelection === "both" ? "default" : "outline"}
            size="sm"
            onClick={() => onHandChange("both")}
            aria-label="Both hands"
            className="rounded-none border-x-0"
          >
            Both
          </Button>
          <Button
            variant={handSelection === "right" ? "default" : "outline"}
            size="sm"
            onClick={() => onHandChange("right")}
            aria-label="Right hand"
            className={
              handSelection === "right"
                ? "bg-right-hand hover:bg-right-hand-dark border-right-hand rounded-l-none"
                : "rounded-l-none"
            }
          >
            R
          </Button>
        </div>
      </div>
    </div>
  );
}
