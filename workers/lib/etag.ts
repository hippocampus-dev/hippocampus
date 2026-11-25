// Conditional requests, RFC 9110 section 13
// https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.2

function weaken(etag: string): string {
  return etag.startsWith("W/") ? etag.slice(2) : etag;
}

export function matchesIfNoneMatch(request: Request, etag: string): boolean {
  const header = request.headers.get("If-None-Match");
  if (header === null) {
    return false;
  }
  if (header.trim() === "*") {
    return true;
  }

  // GET uses the weak comparison function, so W/ prefixes are ignored.
  const target = weaken(etag);
  return header
    .split(",")
    .some((candidate) => weaken(candidate.trim()) === target);
}
