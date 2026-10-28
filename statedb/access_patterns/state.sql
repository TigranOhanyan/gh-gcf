-- name: CreateState :exec
INSERT INTO state (name, value) VALUES (?, ?);

-- name: GetState :one
SELECT * FROM state WHERE name = ? LIMIT 1;

-- name: GetStates :many
SELECT * FROM state;
