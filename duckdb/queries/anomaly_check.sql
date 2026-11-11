.read /opt/hippocampus/duckdb/schemas/claude_anomalies.sql

SELECT
    timestamp,
    project,
    model,
    total_tokens,
    ROUND(z_score, 2) AS z_score
FROM claude_anomalies
WHERE ABS(z_score) > 3
ORDER BY total_tokens DESC;
