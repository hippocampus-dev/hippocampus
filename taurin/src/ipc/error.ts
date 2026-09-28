import type { CommandError } from "./types/error";

function isCommandError(error: unknown): error is CommandError {
  return typeof error === "object" && error !== null && "kind" in error;
}

export function isCancelled(error: unknown): boolean {
  return isCommandError(error) && error.kind === "Cancelled";
}

export function describeCommandError(error: unknown): string {
  if (isCommandError(error)) {
    return error.kind === "Failed" ? error.message : "cancelled";
  }
  return String(error);
}
