import type { Context, MiddlewareHandler, Next } from "hono";
import { createMiddleware } from "hono/factory";
import {
  type GitHubOIDCCondition,
  type GitHubOIDCPayload,
  GitHubOIDCVerifier,
} from "./oidc";
import { problem } from "./problem";
import type { RateLimiter, RateLimitInformation } from "./rate_limit";

export interface RateLimitMiddlewareOptions {
  limiter: RateLimiter;
  limit: number;
  keyFunction: (context: Context) => string;
}

const BEARER_TOKEN_PATTERN = /^Bearer +([A-Za-z0-9._~+/-]+=*) *$/i;

export function githubOIDCMiddleware(
  conditions: GitHubOIDCCondition[],
): MiddlewareHandler {
  const verifier = new GitHubOIDCVerifier(conditions);

  return createMiddleware(async (context: Context, next: Next) => {
    const match = context.req
      .header("Authorization")
      ?.match(BEARER_TOKEN_PATTERN);
    const token = match?.[1] ?? null;

    if (!token) {
      return problem(context, 401, "Missing Authorization header");
    }

    try {
      const payload = await verifier.verify(token);
      context.set("oidcAuth", payload);
      await next();
    } catch (error) {
      console.error("OIDC verification failed:", error);
      return problem(context, 401, "Token verification failed");
    }
  });
}

export function rateLimitMiddleware(
  options: RateLimitMiddlewareOptions,
): MiddlewareHandler {
  return createMiddleware(async (context: Context, next: Next) => {
    const key = options.keyFunction(context);

    const rateLimit = await options.limiter.remaining(key, options.limit);

    setRateLimitHeaders(context, rateLimit);

    if (rateLimit.isExceeded) {
      return problem(context, 429, "Rate limit exceeded");
    }

    await options.limiter.take(key, 1);
    await next();
  });
}

function setRateLimitHeaders(
  context: Context,
  rateLimit: RateLimitInformation,
): void {
  context.header("X-RateLimit-Limit", rateLimit.limit.toString());
  context.header("X-RateLimit-Remaining", rateLimit.remaining.toString());
  context.header("X-RateLimit-Reset", rateLimit.resetTimestamp.toString());

  if (rateLimit.retryAfter !== null) {
    context.header("Retry-After", rateLimit.retryAfter.toString());
  }
}

export function getAuth(context: Context): GitHubOIDCPayload | null {
  return (context.get("oidcAuth") as GitHubOIDCPayload | undefined) ?? null;
}
