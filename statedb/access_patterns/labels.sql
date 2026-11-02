-- name: ListLabels :many
SELECT name, color, registered_at, created_at FROM label ORDER BY name;

-- name: GetLabel :one
SELECT name, color, registered_at, created_at FROM label WHERE name = ?;

-- name: CreateLabel :exec
INSERT INTO label (name, color) VALUES (?, ?);

-- name: SetLabelCreatedAt :exec
UPDATE label SET created_at = ? WHERE name = ?;
