.read /opt/hippocampus/duckdb/schemas/claude_conversations.sql

FROM (
    SELECT
        project,
        REGEXP_REPLACE(model, '-\d{8}$', '') AS model_group,
        total_tokens
    FROM claude_conversations
    WHERE model IS NOT NULL AND project IS NOT NULL
)
PIVOT (
    SUM(total_tokens)
    FOR model_group IN (
        'claude-sonnet-4-5',
        'claude-sonnet-4',
        'claude-haiku-4-5',
        '<synthetic>'
    )
)
ORDER BY project;
