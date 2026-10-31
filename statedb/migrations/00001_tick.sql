-- +goose Up
CREATE TABLE tick (
    id TEXT PRIMARY KEY CHECK (length(id) = 26)
);
