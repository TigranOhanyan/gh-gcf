-- name: ListLabels :many
SELECT name, color, registered_at FROM label ORDER BY name;

-- name: CreateLabel :exec
INSERT INTO label (name, color) VALUES (?, ?);
