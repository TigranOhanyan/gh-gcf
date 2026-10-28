-- +goose Up
CREATE TABLE state (
    name            TEXT    NOT NULL,
    value           TEXT    NOT NULL,
    PRIMARY KEY (name)
);