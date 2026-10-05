// RFC 9457 Problem Details for HTTP APIs
// https://www.rfc-editor.org/rfc/rfc9457.html

export interface ProblemDetails {
  type: string;
  title: string;
  status: number;
  detail?: string;
}

const TITLES: Record<number, string> = {
  400: "Bad Request",
  404: "Not Found",
  413: "Content Too Large",
  500: "Internal Server Error",
};

export function problem(status: number, detail?: string): Response {
  const body: ProblemDetails = {
    // "about:blank" means the status code alone carries the semantics,
    // which is the correct default until a type URI is documented.
    type: "about:blank",
    title: TITLES[status] ?? "Error",
    status,
  };
  if (detail !== undefined) {
    body.detail = detail;
  }

  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/problem+json; charset=utf-8" },
  });
}
