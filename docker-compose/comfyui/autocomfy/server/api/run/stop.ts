import { defineEventHandler, createError } from "h3";
import { getRunState, updateRunState, abortRun } from "@/lib/run-state";
import { broadcast } from "../ws";

export default defineEventHandler(async () => {
  const state = getRunState();
  if (state.status !== "running") {
    throw createError({ statusCode: 409, message: "Not running" });
  }

  updateRunState({ status: "stopping" });
  abortRun();
  broadcast({ type: "run_state", data: getRunState() });

  return { ok: true };
});
