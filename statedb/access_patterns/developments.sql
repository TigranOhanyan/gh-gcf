-- name: GetDevelopment :one
SELECT
    name,
    starting_label,
    attention_label,
    standards_reviewer_app,
    specification_reviewer_app,
    builder_app,
    fixer_app,
    builder_instruction_sha256,
    standards_reviewer_instruction_sha256,
    specification_reviewer_instruction_sha256,
    fixer_instruction_sha256
FROM development
WHERE name = ?;

-- name: CreateDevelopment :exec
INSERT INTO development (
    name,
    starting_label,
    attention_label,
    standards_reviewer_app,
    specification_reviewer_app,
    builder_app,
    fixer_app,
    builder_instruction_sha256,
    standards_reviewer_instruction_sha256,
    specification_reviewer_instruction_sha256,
    fixer_instruction_sha256
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateDevelopment :exec
UPDATE development
SET starting_label = ?,
    attention_label = ?,
    standards_reviewer_app = ?,
    specification_reviewer_app = ?,
    builder_app = ?,
    fixer_app = ?,
    builder_instruction_sha256 = ?,
    standards_reviewer_instruction_sha256 = ?,
    specification_reviewer_instruction_sha256 = ?,
    fixer_instruction_sha256 = ?
WHERE name = ?;

-- name: ListDevelopments :many
SELECT
    name,
    starting_label,
    attention_label,
    standards_reviewer_app,
    specification_reviewer_app,
    builder_app,
    fixer_app,
    builder_instruction_sha256,
    standards_reviewer_instruction_sha256,
    specification_reviewer_instruction_sha256,
    fixer_instruction_sha256
FROM development
ORDER BY name;

-- name: GetDevelopmentCheck :one
SELECT development_name, name FROM development_check WHERE development_name = ? AND name = ?;

-- name: CreateDevelopmentCheck :exec
INSERT INTO development_check (development_name, name) VALUES (?, ?);

-- name: ListDevelopmentChecks :many
SELECT development_name, name FROM development_check ORDER BY development_name, name;

-- name: ListDevelopmentCheckNames :many
SELECT name FROM development_check WHERE development_name = ? ORDER BY name;
