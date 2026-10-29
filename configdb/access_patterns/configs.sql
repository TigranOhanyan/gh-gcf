-- name: CreateConfig :exec
INSERT INTO config (name, value) VALUES (?, ?);

-- name: GetConfig :one
SELECT * FROM config WHERE name = ? LIMIT 1;

-- name: GetConfigs :many
SELECT * FROM config;

-- name: GetAppliedMigrationVersions :many
SELECT version_id FROM goose_db_version WHERE is_applied = 1;
