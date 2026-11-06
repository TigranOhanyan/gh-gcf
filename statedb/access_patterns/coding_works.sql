-- name: GetCodingWorkByIssue :one
SELECT id, issue_number, builder_name, pull_request_number, builder_head_sha, fixer_head_sha, status
FROM coding_work WHERE issue_number = ?;

-- name: ListCodingWorks :many
SELECT id, issue_number, builder_name, pull_request_number, builder_head_sha, fixer_head_sha, status
FROM coding_work ORDER BY issue_number;

-- name: CreateIssue :exec
INSERT INTO issue (number, url) VALUES (?, ?);

-- name: CreateCodingWork :exec
INSERT INTO coding_work (id, issue_number, builder_name) VALUES (?, ?, ?);

-- name: CreateAgent :exec
INSERT INTO agent (id, role, work_id, member_name) VALUES (?, ?, ?, ?);

-- name: SetAgentRunID :exec
UPDATE agent SET cursor_run_id = ? WHERE id = ?;

-- name: ListAgents :many
SELECT id, role, work_id, member_name, cursor_run_id FROM agent ORDER BY id;

-- name: CreateBuilderAgent :exec
INSERT INTO builder_agent (id) VALUES (?);

-- name: ListBuilderAgents :many
SELECT id, role FROM builder_agent ORDER BY id;
