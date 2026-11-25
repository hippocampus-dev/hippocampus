.read /opt/hippocampus/duckdb/schemas/claude_conversations.sql

CREATE OR REPLACE MACRO z_score(value, mean_value, standard_deviation) AS (
    (value - mean_value) / NULLIF(standard_deviation, 0)
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
        statistics.mean_tokens,
        statistics.stddev_tokens
    ) AS z_score
FROM claude_conversations
CROSS JOIN stats statistics
WHERE timestamp IS NOT NULL;
