import { defineEventHandler } from "h3";
import { problem } from "@/server/problem";
import { abortRun, getRunState, updateRunState } from "@/server/run-state";
import { broadcast } from "../ws";

export default defineEventHandler(async () => {
  const state = getRunState();
  if (state.status !== "running") {
    return problem(409, "Not running");
  }

  updateRunState({ status: "stopping" });
  abortRun();
  broadcast({ type: "run_state", data: getRunState() });

  return { ok: true };
});
