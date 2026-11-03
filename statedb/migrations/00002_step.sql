-- +goose Up
CREATE TABLE step (
    id      TEXT PRIMARY KEY CHECK (length(id) = 26),
    tick_id TEXT NOT NULL REFERENCES tick (id),
    name    TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
