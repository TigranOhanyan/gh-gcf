-- name: ListGithubApps :many
SELECT app_id FROM github_app ORDER BY app_id;

-- name: CreateGithubApp :exec
INSERT INTO github_app (app_id) VALUES (?);
