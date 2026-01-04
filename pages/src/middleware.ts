import {createMiddleware} from "hono/factory";
import type {Context, MiddlewareHandler, Next} from "hono";
import type {RateLimiter, RateLimitInfo} from "./rate_limit";

export interface RateLimitMiddlewareOptions {
    limiter: RateLimiter;
    limit: number;
    keyFunction: (context: Context) => string;
}

export function rateLimitMiddleware(
    options: RateLimitMiddlewareOptions
): MiddlewareHandler {
    return createMiddleware(async (context: Context, next: Next) => {
        const key = options.keyFunction(context);

        const info = await options.limiter.remaining(key, options.limit);

        setRateLimitHeaders(context, info);

        if (info.isExceeded) {
            return context.json(
                {
                    error: "Too Many Requests",
                    retryAfter: info.retryAfter,
                },
                429
            );
        }

        await options.limiter.take(key, 1);
        await next();
    });
}

function setRateLimitHeaders(context: Context, info: RateLimitInfo): void {
    context.header("X-RateLimit-Limit", info.limit.toString());
    context.header("X-RateLimit-Remaining", info.remaining.toString());
    context.header("X-RateLimit-Reset", info.resetTimestamp.toString());

    if (info.retryAfter !== null) {
        context.header("Retry-After", info.retryAfter.toString());
    }
}

export function getClientIdentifier(context: Context): string {
    const connectingIp = context.req.header("cf-connecting-ip");
    if (connectingIp) {
        return connectingIp;
    }

    return "unknown";
}
