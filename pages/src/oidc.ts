import * as jose from "jose";

const GITHUB_OIDC_ISSUER = "https://token.actions.githubusercontent.com";
const GITHUB_OIDC_JWKS_URL = new URL(
    "https://token.actions.githubusercontent.com/.well-known/jwks"
);

export interface GitHubOIDCPayload {
    sub: string;
    aud: string;
    iss: string;
    repository: string;
    repository_owner: string;
    actor: string;
    workflow: string;
    ref: string;
    sha: string;
    run_id: string;
    run_number: string;
    run_attempt: string;
    job_workflow_ref: string;
}

export interface GitHubOIDCCondition {
    claim: keyof GitHubOIDCPayload;
    values: string[];
}

export class GitHubOIDCVerifier {
    private jwks: jose.JWTVerifyGetKey;
    private conditions: Array<{claim: keyof GitHubOIDCPayload; values: Set<string>}>;

    constructor(conditions: GitHubOIDCCondition[]) {
        this.jwks = jose.createRemoteJWKSet(GITHUB_OIDC_JWKS_URL, {
            timeoutDuration: 5000,
            cooldownDuration: 30000,
        });
        this.conditions = conditions.map((condition) => ({
            claim: condition.claim,
            values: new Set(condition.values.map((v) => v.toLowerCase())),
        }));
    }

    async verify(token: string): Promise<GitHubOIDCPayload> {
        const {payload} = await jose.jwtVerify(token, this.jwks, {
            issuer: GITHUB_OIDC_ISSUER,
            algorithms: ["RS256"],
            clockTolerance: 5,
        });

        const typedPayload = payload as unknown as GitHubOIDCPayload;

        for (const condition of this.conditions) {
            const value = typedPayload[condition.claim];
            if (typeof value !== "string") {
                throw new Error(`Missing claim: ${condition.claim}`);
            }
            if (!condition.values.has(value.toLowerCase())) {
                console.warn(
                    `Blocked OIDC: ${condition.claim}=${value} not in allowed values`
                );
                throw new Error(`Invalid ${condition.claim}`);
            }
        }

        return typedPayload;
    }
}

