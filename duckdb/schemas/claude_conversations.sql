CREATE OR REPLACE VIEW claude_conversations AS
SELECT
    ROW_NUMBER() OVER () AS id,
    SPLIT_PART(filename, '/', -2) AS project,
    TRY_CAST(timestamp AS TIMESTAMP) AS timestamp,
    type,
    message.model AS model,
    CASE
        WHEN message.content IS NOT NULL AND TRY_CAST(message.content AS VARCHAR) IS NOT NULL THEN TRY_CAST(message.content AS VARCHAR)
        WHEN json_extract_string(message.content, '$[0].text') IS NOT NULL THEN json_extract_string(message.content, '$[0].text')
        ELSE content
    END AS content,
    cwd,
    message.usage.input_tokens AS input_tokens,
    message.usage.output_tokens AS output_tokens,
    COALESCE(message.usage.input_tokens, 0) + COALESCE(message.usage.output_tokens, 0) AS total_tokens,
    message.usage.cache_creation_input_tokens AS cache_creation_input_tokens,
    message.usage.cache_read_input_tokens AS cache_read_input_tokens,
FROM read_json(
    '~/.claude/projects/**/*.jsonl',
    format='newline_delimited',
    union_by_name=true,
    filename=true
);
