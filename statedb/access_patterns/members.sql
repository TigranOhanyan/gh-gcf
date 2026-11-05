-- name: GetMember :one
SELECT name, role, attention_label, instruction_sha256 FROM member WHERE name = ?;

-- name: CreateMember :exec
INSERT INTO member (name, role, attention_label, instruction_sha256) VALUES (?, ?, ?, ?);

-- name: UpdateMember :exec
UPDATE member SET attention_label = ?, instruction_sha256 = ? WHERE name = ?;

-- name: GetBuilderMember :one
SELECT name, role, starting_label FROM builder_member WHERE name = ?;

-- name: CreateBuilderMember :exec
INSERT INTO builder_member (name, starting_label) VALUES (?, ?);

-- name: UpdateBuilderStartingLabel :exec
UPDATE builder_member SET starting_label = ? WHERE name = ?;

-- name: GetReviewerMember :one
SELECT name, role, builder_name, github_app FROM reviewer_member WHERE name = ?;

-- name: CreateReviewerMember :exec
INSERT INTO reviewer_member (name, builder_name, github_app) VALUES (?, ?, ?);

-- name: UpdateReviewerMember :exec
UPDATE reviewer_member SET builder_name = ?, github_app = ? WHERE name = ?;

-- name: GetFixerMember :one
SELECT name, role, builder_name FROM fixer_member WHERE name = ?;

-- name: CreateFixerMember :exec
INSERT INTO fixer_member (name, builder_name) VALUES (?, ?);

-- name: UpdateFixerBuilder :exec
UPDATE fixer_member SET builder_name = ? WHERE name = ?;

-- name: GetGithubCheck :one
SELECT builder_name, name FROM github_check WHERE builder_name = ? AND name = ?;

-- name: CreateGithubCheck :exec
INSERT INTO github_check (builder_name, name) VALUES (?, ?);

-- name: ListBuilderMembers :many
SELECT m.name, m.role, m.attention_label, m.instruction_sha256, b.starting_label
FROM builder_member b
JOIN member m ON m.name = b.name
ORDER BY m.name;

-- name: ListReviewerMembers :many
SELECT m.name, m.role, m.attention_label, m.instruction_sha256, r.builder_name, r.github_app
FROM reviewer_member r
JOIN member m ON m.name = r.name
ORDER BY m.name;

-- name: ListFixerMembers :many
SELECT m.name, m.role, m.attention_label, m.instruction_sha256, f.builder_name
FROM fixer_member f
JOIN member m ON m.name = f.name
ORDER BY m.name;

-- name: ListGithubChecks :many
SELECT builder_name, name FROM github_check ORDER BY builder_name, name;
