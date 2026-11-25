import type { AgentEmail } from "agents";

const TRUSTED_AUTHSERV_ID = "mx.cloudflare.net";

interface EmailAuthenticationResult {
  dmarcPass: boolean;
  headerFrom: string | null;
}

// RFC 5322: Strip comments and folding whitespace, handling quotes and escapes
function stripCFWS(input: string): string | null {
  const MAX_HEADER_LENGTH = 8192;
  if (input.length > MAX_HEADER_LENGTH) {
    return null;
  }

  let result = "";
  let depth = 0;
  let inQuote = false;
  let escaped = false;
  let previousWasSpace = false;

  for (const char of input) {
    if (escaped) {
      if (depth === 0) {
        result += char;
        previousWasSpace = false;
      }
      escaped = false;
      continue;
    }

    if (char === "\\") {
      escaped = true;
      if (depth === 0) {
        result += char;
        previousWasSpace = false;
      }
      continue;
    }

    if (char === '"' && depth === 0) {
      inQuote = !inQuote;
      result += char;
      previousWasSpace = false;
      continue;
    }

    if (!inQuote) {
      if (char === "(") {
        if (depth === 0 && !previousWasSpace) {
          result += " ";
          previousWasSpace = true;
        }
        depth++;
        continue;
      }
      if (char === ")") {
        if (depth === 0) {
          return null;
        }
        depth--;
        continue;
      }
    }

    if (depth === 0) {
      if (inQuote) {
        result += char;
        previousWasSpace = false;
      } else if (/\s/.test(char)) {
        if (!previousWasSpace) {
          result += " ";
          previousWasSpace = true;
        }
      } else {
        result += char;
        previousWasSpace = false;
      }
    }
  }

  if (depth !== 0 || inQuote || escaped) {
    return null;
  }

  return result.trim();
}

function extractTrustedSection(header: string): string | null {
  const stripped = stripCFWS(header);
  if (stripped === null) {
    return null;
  }

  let mostRecentHeader = stripped;
  let inQuote = false;
  let escaped = false;
  for (let i = 0; i < stripped.length; i++) {
    const char = stripped[i];
    if (escaped) {
      escaped = false;
      continue;
    }
    if (char === "\\") {
      escaped = true;
      continue;
    }
    if (char === '"') {
      inQuote = !inQuote;
    } else if (char === "," && !inQuote) {
      mostRecentHeader = stripped.slice(0, i).trim();
      break;
    }
  }

  // RFC 8617 (ARC): "i=1; authserv-id; method=result; ..."
  const normalized = mostRecentHeader
    .replace(/^i\s*=\s*\d+\s*;\s*/i, "")
    .trimStart()
    .toLowerCase();

  if (!normalized.startsWith(TRUSTED_AUTHSERV_ID)) {
    return null;
  }

  const afterAuthservId = normalized
    .slice(TRUSTED_AUTHSERV_ID.length)
    .trimStart();
  // RFC 8601: authserv-id [CFWS authres-version] ";"
  const afterVersion = afterAuthservId.replace(/^\d+\s*/, "");
  if (!afterVersion.startsWith(";")) {
    return null;
  }

  return afterVersion.slice(1);
}

export function verifyEmailAuthentication(
  message: AgentEmail,
): EmailAuthenticationResult {
  const header =
    message.headers.get("Authentication-Results") ??
    message.headers.get("Arc-Authentication-Results") ??
    "";

  const trustedSection = extractTrustedSection(header);
  if (trustedSection !== null) {
    const dmarc = trustedSection.match(
      /\bdmarc\s*=\s*pass\b[^;]*\bheader\.from\s*=\s*"?([^\s;"]+)/i,
    );
    return {
      dmarcPass: dmarc !== null,
      headerFrom: dmarc !== null ? dmarc[1] : null,
    };
  }

  // Cloudflare Email Routing enforces SPF/DKIM but not From alignment, so a missing DMARC result cannot rule out a spoofed From and is rejected.
  return { dmarcPass: false, headerFrom: null };
}
