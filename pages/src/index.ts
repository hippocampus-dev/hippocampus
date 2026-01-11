import {Hono} from "hono";
import {KVSlidingRateLimiter} from "./rate_limit";
import {getClientIdentifier, rateLimitMiddleware} from "./middleware";

const application = new Hono<{ Bindings: CloudflareEnvironment }>();

application.get("/healthz", (context) => {
    return context.text("OK");
});

application.use("/api/*", async (context, next) => {
    const limiter = new KVSlidingRateLimiter(context.env.RATE_LIMIT_KV, 60);

    const middleware = rateLimitMiddleware({
        limiter,
        limit: 10,
        keyFunction: getClientIdentifier,
    });

    return middleware(context, next);
});

export default application;
