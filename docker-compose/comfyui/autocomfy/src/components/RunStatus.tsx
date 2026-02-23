import { Badge } from "@/components/ui/badge";
import { Progress } from "@/components/ui/progress";
import type { RunState } from "@/lib/types";

interface RunStatusProps {
  runState: RunState;
  connected: boolean;
}

export default function RunStatus({ runState, connected }: RunStatusProps) {
  const statusColor = {
    idle: "bg-gray-500",
    running: "bg-green-500",
    stopping: "bg-yellow-500",
  }[runState.status];

  const progressPercent =
    runState.progress && runState.progress.total > 0
      ? (runState.progress.current / runState.progress.total) * 100
      : 0;

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-3">
        <Badge variant="outline" className="gap-1.5">
          <span
            className={`h-2 w-2 rounded-full ${statusColor}`}
          />
          {runState.status}
        </Badge>

        {!connected && (
          <Badge variant="destructive">Disconnected</Badge>
        )}

        {runState.workflowName && (
          <Badge variant="secondary">{runState.workflowName}</Badge>
        )}
      </div>

      {runState.status !== "idle" && (
        <>
          <div className="text-sm text-muted-foreground">
            Completed: {runState.completedCount}
            {runState.mode === "count" && ` / ${runState.targetCount}`}
          </div>

          {runState.progress && (
            <div className="space-y-1">
              <Progress value={progressPercent} className="h-2" />
              <div className="text-xs text-muted-foreground">
                Step {runState.progress.current} / {runState.progress.total}
              </div>
            </div>
          )}
        </>
      )}

      {runState.errors.length > 0 && (
        <div className="space-y-1">
          {runState.errors.slice(-3).map((error, i) => (
            <div
              key={i}
              className="text-sm text-destructive bg-destructive/10 rounded p-2"
            >
              {error}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
