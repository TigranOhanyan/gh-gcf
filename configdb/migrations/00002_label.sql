-- +goose Up
CREATE TABLE label (
    name           TEXT PRIMARY KEY,
    color          TEXT NOT NULL,
    registered_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
