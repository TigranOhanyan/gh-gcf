-- name: ListBuilderMembers :many
SELECT m.name, m.attention_label, b.starting_label
FROM builder_member b
JOIN member m ON m.name = b.name
ORDER BY m.name;

-- name: ListStartingLabels :many
SELECT starting_label FROM builder_member ORDER BY starting_label;

-- name: ListReviewerMembers :many
SELECT m.name, m.attention_label, r.github_app
FROM reviewer_member r
JOIN member m ON m.name = r.name
WHERE r.builder_name = ?
ORDER BY m.name;

-- name: ListReviewerApps :many
SELECT github_app FROM reviewer_member ORDER BY github_app;

-- name: ListFixerMembers :many
SELECT m.name, m.attention_label
FROM fixer_member f
JOIN member m ON m.name = f.name
WHERE f.builder_name = ?
ORDER BY m.name;

-- name: ListMemberNames :many
SELECT name FROM member ORDER BY name;

-- name: CreateMember :exec
INSERT INTO member (name, role, attention_label) VALUES (?, ?, ?);

-- name: CreateBuilderMember :exec
INSERT INTO builder_member (name, starting_label) VALUES (?, ?);

-- name: CreateReviewerMember :exec
INSERT INTO reviewer_member (name, builder_name, github_app) VALUES (?, ?, ?);

-- name: CreateFixerMember :exec
INSERT INTO fixer_member (name, builder_name) VALUES (?, ?);

-- name: ListGithubChecks :many
SELECT name FROM github_check WHERE builder_name = ? ORDER BY name;

-- name: CreateGithubCheck :exec
INSERT INTO github_check (builder_name, name) VALUES (?, ?);
