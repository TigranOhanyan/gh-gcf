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

-- name: SetDevelopmentWorkPull :exec
UPDATE development_work
SET pull_request_number = ?, builder_head_sha = ?
WHERE id = ?;

-- name: ListPendingDevelopmentBuilders :many
SELECT
    w.id AS work_id,
    w.development_name,
    w.issue_number,
    a.id AS agent_id,
    a.cursor_run_id
FROM development_work w
JOIN development_agent a ON a.id = w.builder_agent
WHERE w.status = ''
AND a.role = 'builder'
AND a.cursor_run_id IS NOT NULL
AND a.cursor_run_id != ''
AND a.finished_at IS NULL
AND EXISTS (
    SELECT 1 FROM development_event e
    WHERE e.agent_id = a.id
    AND e.kind = 'ack'
)
AND NOT EXISTS (
    SELECT 1 FROM development_event e
    WHERE e.agent_id = a.id
    AND e.kind IN ('completed', 'failed', 'rejected')
)
ORDER BY w.issue_number;

-- name: CreateDevelopmentAgent :exec
INSERT INTO development_agent (id, role) VALUES (?, ?);

-- name: SetDevelopmentAgentRunID :exec
UPDATE development_agent SET cursor_run_id = ? WHERE id = ?;

-- name: ListReadyStandardsWorks :many
SELECT
    w.id AS work_id,
    w.development_name,
    w.issue_number,
    w.issue_url,
    w.pull_request_number
FROM development_work w
JOIN development_agent a ON a.id = w.builder_agent
WHERE w.status = ''
AND w.standards_reviewer_agent IS NULL
AND a.role = 'builder'
AND a.finished_at IS NOT NULL
ORDER BY w.issue_number;

-- name: SetStandardsReviewerAgent :exec
UPDATE development_work
SET standards_reviewer_agent = ?
WHERE id = ?;

-- name: ListReadySpecificationWorks :many
SELECT
    w.id AS work_id,
    w.development_name,
    w.issue_number,
    w.issue_url,
    w.pull_request_number
FROM development_work w
JOIN development_agent a ON a.id = w.standards_reviewer_agent
WHERE w.status = ''
AND w.specification_reviewer_agent IS NULL
AND a.role = 'standards_reviewer'
AND a.finished_at IS NOT NULL
ORDER BY w.issue_number;

-- name: SetSpecificationReviewerAgent :exec
UPDATE development_work
SET specification_reviewer_agent = ?
WHERE id = ?;

-- name: SetDevelopmentAgentFinished :exec
UPDATE development_agent
SET finished_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE id = ?;

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
