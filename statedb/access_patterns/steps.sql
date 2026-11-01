-- name: CreateStep :exec
INSERT INTO step (id, tick_id, name) VALUES (?, ?, ?);

-- name: GetSteps :many
SELECT * FROM step ORDER BY id;
