export interface RateLimitInfo {
    limit: number;
    remaining: number;
    resetTimestamp: number;
    retryAfter: number | null;
    isExceeded: boolean;
}

interface BucketedRateLimitData {
    buckets: Record<string, number>;
    version: number;
}

export abstract class RateLimiter {
    abstract take(key: string, amount: number): Promise<void>;

    abstract remaining(key: string, limit: number): Promise<RateLimitInfo>;
}

export class KVFixedRateLimiter extends RateLimiter {
    private kvNamespace: KVNamespace;
    private intervalSeconds: number;

    constructor(kvNamespace: KVNamespace, intervalSeconds: number) {
        super();
        this.kvNamespace = kvNamespace;
        this.intervalSeconds = intervalSeconds;
    }

    async remaining(key: string, limit: number): Promise<RateLimitInfo> {
        const redisKey = this.kvKey(key);
        const now = Math.floor(Date.now() / 1000);

        const result = await this.kvNamespace.getWithMetadata<{ expiresAt: number }>(
            redisKey,
            {cacheTtl: 30}
        );

        if (result.value === null) {
            return {
                limit,
                remaining: limit,
                resetTimestamp: now + this.intervalSeconds,
                retryAfter: null,
                isExceeded: false,
            };
        }

        const usedTokens = parseInt(result.value, 10);
        if (Number.isNaN(usedTokens)) {
            return {
                limit,
                remaining: limit,
                resetTimestamp: now + this.intervalSeconds,
                retryAfter: null,
                isExceeded: false,
            };
        }
        const remainingTokens = Math.max(0, limit - usedTokens);

        const expiresAt =
            typeof result.metadata?.expiresAt === "number"
                ? result.metadata.expiresAt
                : now + this.intervalSeconds;
        const resetTimestamp = expiresAt;

        let retryAfterValue: number | null = null;
        if (remainingTokens <= 0) {
            retryAfterValue = Math.max(0, resetTimestamp - now);
        }

        return {
            limit,
            remaining: remainingTokens,
            resetTimestamp,
            retryAfter: retryAfterValue,
            isExceeded: remainingTokens <= 0,
        };
    }

    async take(key: string, amount: number): Promise<void> {
        if (amount <= 0) {
            return;
        }

        const redisKey = this.kvKey(key);
        const now = Math.floor(Date.now() / 1000);
        const expiresAt = now + this.intervalSeconds;

        const currentValue = await this.kvNamespace.get(redisKey, {cacheTtl: 30});

        let newValue: number;
        if (currentValue === null) {
            newValue = amount;
        } else {
            newValue = parseInt(currentValue, 10) + amount;
        }

        await this.kvNamespace.put(redisKey, newValue.toString(), {
            expirationTtl: this.intervalSeconds,
            metadata: {expiresAt},
        });
    }

    private kvKey(key: string): string {
        return `${key}:ratelimit:fixed`;
    }
}

export class KVSlidingRateLimiter extends RateLimiter {
    private kvNamespace: KVNamespace;
    private intervalSeconds: number;
    private bucketSizeSeconds: number;

    constructor(
        kvNamespace: KVNamespace,
        intervalSeconds: number,
        bucketSizeSeconds: number = 1
    ) {
        super();
        this.kvNamespace = kvNamespace;
        this.intervalSeconds = intervalSeconds;
        this.bucketSizeSeconds = bucketSizeSeconds;
    }

    async remaining(key: string, limit: number): Promise<RateLimitInfo> {
        const kvKey = this.kvKey(key);
        const now = Math.floor(Date.now() / 1000);
        const windowStart = now - this.intervalSeconds;

        const data = await this.getData(kvKey);
        const prunedBuckets = this.pruneBuckets(data.buckets, windowStart);

        const usedTokens = Object.values(prunedBuckets).reduce(
            (sum, count) => sum + count,
            0
        );
        const remainingTokens = Math.max(0, limit - usedTokens);

        const bucketNumbers = Object.keys(prunedBuckets).map((bucket) =>
            parseInt(bucket, 10)
        );
        const oldestBucket =
            bucketNumbers.length > 0
                ? Math.min(...bucketNumbers)
                : Math.floor(now / this.bucketSizeSeconds);
        const resetTimestamp =
            oldestBucket * this.bucketSizeSeconds + this.intervalSeconds;

        let retryAfterValue: number | null = null;
        if (remainingTokens <= 0) {
            retryAfterValue = Math.max(0, resetTimestamp - now);
        }

        return {
            limit,
            remaining: remainingTokens,
            resetTimestamp,
            retryAfter: retryAfterValue,
            isExceeded: remainingTokens <= 0,
        };
    }

    async take(key: string, amount: number): Promise<void> {
        if (amount <= 0) {
            return;
        }

        const kvKey = this.kvKey(key);
        const now = Math.floor(Date.now() / 1000);
        const windowStart = now - this.intervalSeconds;
        const currentBucket = Math.floor(now / this.bucketSizeSeconds);

        const data = await this.getData(kvKey);
        const prunedBuckets = this.pruneBuckets(data.buckets, windowStart);

        const bucketKey = currentBucket.toString();
        prunedBuckets[bucketKey] = (prunedBuckets[bucketKey] ?? 0) + amount;

        const newData: BucketedRateLimitData = {
            buckets: prunedBuckets,
            version: data.version + 1,
        };

        await this.kvNamespace.put(kvKey, JSON.stringify(newData), {
            expirationTtl: this.intervalSeconds + this.bucketSizeSeconds,
        });
    }

    private async getData(kvKey: string): Promise<BucketedRateLimitData> {
        const raw = await this.kvNamespace.get(kvKey, {cacheTtl: 30});
        if (raw === null) {
            return {buckets: {}, version: 0};
        }
        try {
            const parsed = JSON.parse(raw) as BucketedRateLimitData;
            if (
                typeof parsed === "object" &&
                parsed !== null &&
                typeof parsed.buckets === "object" &&
                typeof parsed.version === "number"
            ) {
                return parsed;
            }
            return {buckets: {}, version: 0};
        } catch {
            return {buckets: {}, version: 0};
        }
    }

    private pruneBuckets(
        buckets: Record<string, number>,
        windowStart: number
    ): Record<string, number> {
        const windowStartBucket = Math.floor(windowStart / this.bucketSizeSeconds);
        const result: Record<string, number> = {};

        for (const [bucket, count] of Object.entries(buckets)) {
            const bucketNumber = parseInt(bucket, 10);
            if (bucketNumber >= windowStartBucket) {
                result[bucket] = count;
            }
        }

        return result;
    }

    private kvKey(key: string): string {
        return `${key}:ratelimit:sliding`;
    }
}
