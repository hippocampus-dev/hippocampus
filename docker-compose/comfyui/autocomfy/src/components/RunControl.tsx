import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import { Play, Square } from "lucide-react";
import type { RunState } from "@/lib/types";

interface RunControlProps {
  runState: RunState;
  mode: "infinite" | "count";
  count: number;
  onModeChange: (mode: "infinite" | "count") => void;
  onCountChange: (count: number) => void;
  onStart: () => void;
  onStop: () => void;
  canStart: boolean;
}

export default function RunControl({
  runState,
  mode,
  count,
  onModeChange,
  onCountChange,
  onStart,
  onStop,
  canStart,
}: RunControlProps) {
  const isRunning = runState.status === "running";
  const isStopping = runState.status === "stopping";

  return (
    <div className="space-y-4">
      <RadioGroup
        value={mode}
        onValueChange={(value) =>
          onModeChange(value as "infinite" | "count")
        }
        disabled={isRunning || isStopping}
        className="flex gap-4"
      >
        <div className="flex items-center space-x-2">
          <RadioGroupItem value="infinite" id="mode-infinite" />
          <Label htmlFor="mode-infinite">Infinite</Label>
        </div>
        <div className="flex items-center space-x-2">
          <RadioGroupItem value="count" id="mode-count" />
          <Label htmlFor="mode-count">Count</Label>
        </div>
      </RadioGroup>

      {mode === "count" && (
        <Input
          type="number"
          min={1}
          value={count}
          onChange={(e) => onCountChange(Math.max(1, Number(e.target.value)))}
          disabled={isRunning || isStopping}
          placeholder="Number of runs"
          className="w-32"
        />
      )}

      <div className="flex gap-2">
        {!isRunning && !isStopping ? (
          <Button onClick={onStart} disabled={!canStart}>
            <Play className="mr-2 h-4 w-4" />
            Start
          </Button>
        ) : (
          <Button
            onClick={onStop}
            variant="destructive"
            disabled={isStopping}
          >
            <Square className="mr-2 h-4 w-4" />
            {isStopping ? "Stopping..." : "Stop"}
          </Button>
        )}
      </div>
    </div>
  );
}
