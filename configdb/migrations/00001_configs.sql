-- +goose Up
CREATE TABLE config (
    name            TEXT    NOT NULL,
    value           TEXT    NOT NULL,
    PRIMARY KEY (name)
);