// RFC 9457 Problem Details for HTTP APIs
// https://www.rfc-editor.org/rfc/rfc9457.html

import type { Context } from "hono";
import type { ContentfulStatusCode } from "hono/utils/http-status";

const TITLES: Record<number, string> = {
  401: "Unauthorized",
  429: "Too Many Requests",
};

export function problem(
  context: Context,
  status: ContentfulStatusCode,
  detail: string,
): Response {
  return context.json(
    {
      // "about:blank" means the status code alone carries the semantics,
      // which is the correct default until a type URI is documented.
      type: "about:blank",
      title: TITLES[status] ?? "Error",
      status,
      detail,
    },
    status,
    { "Content-Type": "application/problem+json; charset=utf-8" },
  );
}
