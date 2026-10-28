import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { WorkflowInfo } from "@/lib/types";

interface WorkflowSelectorProps {
  workflows: WorkflowInfo[];
  value: string;
  onChange: (workflow: WorkflowInfo) => void;
  disabled?: boolean;
}

export default function WorkflowSelector({
  workflows,
  value,
  onChange,
  disabled,
}: WorkflowSelectorProps) {
  return (
    <Select
      value={value}
      onValueChange={(filename) => {
        const workflow = workflows.find((w) => w.filename === filename);
        if (workflow) onChange(workflow);
      }}
      disabled={disabled}
    >
      <SelectTrigger className="w-full">
        <SelectValue placeholder="Select a workflow" />
      </SelectTrigger>
      <SelectContent>
        {workflows.map((workflow) => (
          <SelectItem key={workflow.filename} value={workflow.filename}>
            {workflow.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
