import { useState, useEffect } from "react";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { fetchWorkflows } from "@/server/functions";
import type { WorkflowInfo } from "@/lib/types";

interface WorkflowSelectorProps {
  value: string;
  onChange: (workflow: WorkflowInfo) => void;
  disabled?: boolean;
}

export default function WorkflowSelector({
  value,
  onChange,
  disabled,
}: WorkflowSelectorProps) {
  const [workflows, setWorkflows] = useState<WorkflowInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchWorkflows()
      .then((data) => {
        setWorkflows(data);
        setLoading(false);
      })
      .catch((e) => {
        setError(e instanceof Error ? e.message : "Failed to load workflows");
        setLoading(false);
      });
  }, []);

  return (
    <>
      <Select
        value={value}
        onValueChange={(filename) => {
          const workflow = workflows.find((w) => w.filename === filename);
          if (workflow) onChange(workflow);
        }}
        disabled={disabled || loading}
      >
        <SelectTrigger className={`w-full ${error ? "border-destructive" : ""}`}>
          <SelectValue
            placeholder={
              loading
                ? "Loading workflows..."
                : error
                  ? "Failed to load workflows"
                  : "Select a workflow"
            }
          />
        </SelectTrigger>
        <SelectContent>
          {workflows.map((workflow) => (
            <SelectItem key={workflow.filename} value={workflow.filename}>
              {workflow.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {error && (
        <p className="text-sm text-destructive mt-1">{error}</p>
      )}
    </>
  );
}
