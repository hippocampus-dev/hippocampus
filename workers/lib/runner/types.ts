export interface RunOutput {
  level: "log" | "warn" | "error" | "exception";
  message: string;
}

export interface RunResult {
  output: RunOutput[];
  error: string | null;
  wallTimeMs: number;
  timedOut: boolean;
}

export interface CodeRunner {
  run(code: string, language: string): Promise<RunResult>;
}
