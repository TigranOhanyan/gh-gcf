-- name: ListGithubApps :many
SELECT app_id, client_id, installation_id FROM github_app ORDER BY app_id;

-- name: GetGithubApp :one
SELECT app_id, client_id, installation_id FROM github_app WHERE app_id = ?;

-- name: CreateGithubApp :exec
INSERT INTO github_app (app_id, client_id, installation_id) VALUES (?, ?, ?);

-- name: UpdateGithubApp :exec
UPDATE github_app SET client_id = ?, installation_id = ? WHERE app_id = ?;
