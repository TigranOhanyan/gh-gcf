-- +goose Up
CREATE TABLE tick (
    id         TEXT PRIMARY KEY CHECK (length(id) = 26),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
