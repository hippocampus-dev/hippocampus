.read /opt/hippocampus/duckdb/schemas/claude_conversations.sql

CREATE OR REPLACE MACRO z_score(val, mean_val, std_val) AS (
    (val - mean_val) / NULLIF(std_val, 0)
);

CREATE OR REPLACE VIEW claude_anomalies AS
WITH stats AS (
    SELECT
        AVG(total_tokens) AS mean_tokens,
        STDDEV(total_tokens) AS stddev_tokens
    FROM claude_conversations
)
SELECT
    id,
    project,
    timestamp,
    model,
    input_tokens,
    output_tokens,
    total_tokens,
    z_score(
        total_tokens,
        s.mean_tokens,
        s.stddev_tokens
    ) AS z_score
FROM claude_conversations
CROSS JOIN stats s
WHERE timestamp IS NOT NULL;
