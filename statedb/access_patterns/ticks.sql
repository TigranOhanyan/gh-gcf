-- name: CreateTick :exec
INSERT INTO tick (id) VALUES (?);

-- name: GetTicks :many
SELECT * FROM tick ORDER BY id;
