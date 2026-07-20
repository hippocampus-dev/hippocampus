.read /opt/hippocampus/duckdb/schemas/claude_conversations.sql

SELECT
    timestamp,
    project,
    model,
    total_tokens,
    input_tokens,
    output_tokens
FROM claude_conversations
ORDER BY total_tokens DESC
LIMIT 20;
