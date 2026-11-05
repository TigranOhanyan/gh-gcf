-- +goose Up
-- Mirror of config members. Label and App references are not enforced here.
-- The app row appears when GitHub answers.

CREATE TABLE member (
    name            TEXT PRIMARY KEY,
    role            TEXT NOT NULL CHECK (role IN ('builder', 'reviewer', 'fixer')),
    attention_label TEXT NOT NULL,
    UNIQUE (name, role)
);

CREATE TABLE builder_member (
    name           TEXT PRIMARY KEY,
    role           TEXT NOT NULL DEFAULT 'builder' CHECK (role = 'builder'),
    starting_label TEXT NOT NULL,
    FOREIGN KEY (name) REFERENCES member (name)
);

CREATE TABLE reviewer_member (
    name         TEXT PRIMARY KEY,
    role         TEXT NOT NULL DEFAULT 'reviewer' CHECK (role = 'reviewer'),
    builder_name TEXT NOT NULL REFERENCES builder_member (name),
    github_app   INTEGER NOT NULL,
    FOREIGN KEY (name) REFERENCES member (name)
);

CREATE TABLE fixer_member (
    name         TEXT PRIMARY KEY,
    role         TEXT NOT NULL DEFAULT 'fixer' CHECK (role = 'fixer'),
    builder_name TEXT NOT NULL UNIQUE REFERENCES builder_member (name),
    FOREIGN KEY (name) REFERENCES member (name)
);

CREATE TABLE github_check (
    builder_name TEXT NOT NULL REFERENCES builder_member (name),
    name         TEXT NOT NULL,
    PRIMARY KEY (builder_name, name)
);
