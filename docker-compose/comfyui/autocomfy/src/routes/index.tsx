import { createFileRoute } from "@tanstack/react-router";
import { useState, useCallback } from "react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Textarea } from "@/components/ui/textarea";
import WorkflowSelector from "@/components/WorkflowSelector";
import RunControl from "@/components/RunControl";
import RunStatus from "@/components/RunStatus";
import ResultGallery from "@/components/ResultGallery";
import { useAutoRun } from "@/hooks/use-auto-run";
import {
  fetchWorkflowData,
  fetchWorkflowTopologyHash,
} from "@/server/functions";
import type { WorkflowInfo } from "@/lib/types";

export const Route = createFileRoute("/")({ component: Dashboard });

function Dashboard() {
  const { runState, connected, start, stop } = useAutoRun();
  const [selectedWorkflow, setSelectedWorkflow] =
    useState<WorkflowInfo | null>(null);
  const [workflowData, setWorkflowData] = useState<unknown>(null);
  const [topologyHash, setTopologyHash] = useState<string | null>(null);
  const [concept, setConcept] = useState("");
  const [mode, setMode] = useState<"infinite" | "count">("infinite");
  const [count, setCount] = useState(1);

  const handleWorkflowChange = useCallback(async (workflow: WorkflowInfo) => {
    setSelectedWorkflow(workflow);
    setTopologyHash(null);
    try {
      const data = await fetchWorkflowData({
        data: { filename: workflow.filename },
      });
      setWorkflowData(data);
      const hash = await fetchWorkflowTopologyHash({
        data: { workflow: data },
      });
      setTopologyHash(hash ?? null);
    } catch {
      // Workflow data will be fetched from ComfyUI directly when starting
    }
  }, []);

  const handleStart = useCallback(async () => {
    if (selectedWorkflow === null) return;

    let workflow = workflowData;
    if (workflow == null) {
      try {
        workflow = await fetchWorkflowData({
          data: { filename: selectedWorkflow.filename },
        });
      } catch {}
    }

    if (workflow == null) return;

    await start({
      workflow,
      workflowName: selectedWorkflow.name,
      mode,
      count: mode === "count" ? count : undefined,
      concept: concept || undefined,
      topologyHash: topologyHash || undefined,
    });
  }, [selectedWorkflow, workflowData, topologyHash, concept, mode, count, start]);

  const isRunning =
    runState.status === "running" || runState.status === "stopping";

  return (
    <div className="min-h-screen bg-background">
      <main className="mx-auto px-4 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-[theme(spacing.96)_1fr] gap-6">
          <div className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle>Workflow</CardTitle>
              </CardHeader>
              <CardContent>
                <WorkflowSelector
                  value={selectedWorkflow?.filename ?? ""}
                  onChange={handleWorkflowChange}
                  disabled={isRunning}
                />
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Concept</CardTitle>
              </CardHeader>
              <CardContent>
                <Textarea
                  value={concept}
                  onChange={(e) => setConcept(e.target.value)}
                  placeholder="Describe the concept for prompt generation..."
                  rows={3}
                  disabled={isRunning}
                />
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Run Control</CardTitle>
              </CardHeader>
              <CardContent>
                <RunControl
                  runState={runState}
                  mode={mode}
                  count={count}
                  onModeChange={setMode}
                  onCountChange={setCount}
                  onStart={handleStart}
                  onStop={stop}
                  canStart={selectedWorkflow !== null && connected}
                />
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Status</CardTitle>
              </CardHeader>
              <CardContent>
                <RunStatus runState={runState} connected={connected} />
              </CardContent>
            </Card>
          </div>

          <div>
            {topologyHash ? (
              <ResultGallery
                refreshKey={runState.completedCount}
                topologyHash={topologyHash}
              />
            ) : null}
          </div>
        </div>
      </main>
    </div>
  );
}
