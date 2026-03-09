import {Hono} from "hono";
import {KVSlidingRateLimiter} from "./rate_limit";
import {getAuth, githubOIDCMiddleware, rateLimitMiddleware} from "./middleware";

const application = new Hono<{
    Bindings: CloudflareEnvironment;
    Variables: { oidcAuth: import("./oidc").GitHubOIDCPayload };
}>();

application.get("/healthz", (context) => {
    return context.text("OK");
});

application.use(
    "/api/*",
    githubOIDCMiddleware([
        {
            claim: "repository_owner", values: ["kaidotio"]
        },
    ])
);

application.use("/api/*", async (context, next) => {
    const limiter = new KVSlidingRateLimiter(context.env.RATE_LIMIT_KV, 60);

    const middleware = rateLimitMiddleware({
        limiter,
        limit: 10,
        keyFunction: (context) => getAuth(context)!.sub,
    });

    return middleware(context, next);
});

export default application;
