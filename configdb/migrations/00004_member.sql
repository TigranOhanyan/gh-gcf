-- +goose Up
-- Shared name, role, and attention label. The child fixes the role.
CREATE TABLE member (
    name            TEXT PRIMARY KEY,
    role            TEXT NOT NULL CHECK (role IN ('builder', 'reviewer', 'fixer')),
    attention_label TEXT NOT NULL REFERENCES label (name),
    UNIQUE (name, role)
);

CREATE TABLE builder_member (
    name           TEXT PRIMARY KEY,
    role           TEXT NOT NULL DEFAULT 'builder' CHECK (role = 'builder'),
    starting_label TEXT NOT NULL REFERENCES label (name),
    FOREIGN KEY (name, role) REFERENCES member (name, role)
);

-- One builder per starting label.
CREATE UNIQUE INDEX builder_member_starting_label ON builder_member (starting_label);

CREATE TABLE reviewer_member (
    name         TEXT PRIMARY KEY,
    role         TEXT NOT NULL DEFAULT 'reviewer' CHECK (role = 'reviewer'),
    builder_name TEXT NOT NULL REFERENCES builder_member (name),
    github_app   INTEGER NOT NULL REFERENCES github_app (app_id),
    FOREIGN KEY (name, role) REFERENCES member (name, role)
);

-- Two reviewers do not share an App.
CREATE UNIQUE INDEX reviewer_member_app ON reviewer_member (github_app);

CREATE TABLE fixer_member (
    name         TEXT PRIMARY KEY,
    role         TEXT NOT NULL DEFAULT 'fixer' CHECK (role = 'fixer'),
    builder_name TEXT NOT NULL UNIQUE REFERENCES builder_member (name),
    FOREIGN KEY (name, role) REFERENCES member (name, role)
);

-- The check run name GitHub shows on the pull request head.
-- A builder with no rows has no CI.
CREATE TABLE github_check (
    builder_name TEXT NOT NULL REFERENCES builder_member (name),
    name         TEXT NOT NULL,
    PRIMARY KEY (builder_name, name)
);
