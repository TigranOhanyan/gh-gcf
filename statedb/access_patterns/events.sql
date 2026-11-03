-- name: CreateEvent :exec
INSERT INTO event (id, step_id, kind, message) VALUES (?, ?, ?, ?);

-- name: GetEvents :many
SELECT * FROM event ORDER BY id;
