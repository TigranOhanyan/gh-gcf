-- +goose Up
CREATE TABLE event (
    id         TEXT PRIMARY KEY CHECK (length(id) = 26),
    step_id    TEXT NOT NULL REFERENCES step (id),
    kind       TEXT NOT NULL CHECK (kind IN ('starting', 'ack', 'rejected', 'completed', 'failed')),
    message    TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
