-- +goose Up
CREATE TABLE github_app (
    app_id          INTEGER PRIMARY KEY,
    client_id       TEXT    NOT NULL,
    installation_id INTEGER NOT NULL
);
