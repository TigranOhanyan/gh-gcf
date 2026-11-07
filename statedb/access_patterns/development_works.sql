-- name: GetDevelopmentWorkByIssue :one
SELECT
    id,
    development_name,
    issue_number,
    issue_url,
    pull_request_number,
    builder_head_sha,
    fixer_head_sha,
    builder_agent,
    standards_reviewer_agent,
    specification_reviewer_agent,
    fixer_agent,
    status
FROM development_work
WHERE issue_number = ?;

-- name: ListDevelopmentWorks :many
SELECT
    id,
    development_name,
    issue_number,
    issue_url,
    pull_request_number,
    builder_head_sha,
    fixer_head_sha,
    builder_agent,
    standards_reviewer_agent,
    specification_reviewer_agent,
    fixer_agent,
    status
FROM development_work
ORDER BY issue_number;

-- name: CreateDevelopmentWork :exec
INSERT INTO development_work (
    id,
    development_name,
    issue_number,
    issue_url,
    builder_agent
) VALUES (?, ?, ?, ?, ?);

-- name: SetDevelopmentWorkAttention :exec
UPDATE development_work SET status = 'attention' WHERE id = ?;

-- name: CreateDevelopmentAgent :exec
INSERT INTO development_agent (id, role) VALUES (?, ?);

-- name: SetDevelopmentAgentRunID :exec
UPDATE development_agent SET cursor_run_id = ? WHERE id = ?;

-- name: ListDevelopmentAgents :many
SELECT id, role, cursor_run_id, started_at, finished_at
FROM development_agent
ORDER BY id;

-- name: CreateDevelopmentEvent :exec
INSERT INTO development_event (id, step_id, agent_id, kind, message) VALUES (?, ?, ?, ?, ?);

-- name: ListDevelopmentEvents :many
SELECT id, step_id, agent_id, kind, message, created_at
FROM development_event
ORDER BY id;
