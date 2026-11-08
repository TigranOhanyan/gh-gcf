-- +goose Up
-- One configured flow. The four roles are fixed and are not columns.
CREATE TABLE development (
    name                       TEXT PRIMARY KEY,
    starting_label             TEXT NOT NULL REFERENCES label (name),
    attention_label            TEXT NOT NULL REFERENCES label (name),
    standards_reviewer_app     INTEGER NOT NULL REFERENCES github_app (app_id),
    specification_reviewer_app INTEGER NOT NULL REFERENCES github_app (app_id),
    CHECK (standards_reviewer_app != specification_reviewer_app)
);

-- One development per starting label.
CREATE UNIQUE INDEX development_starting_label ON development (starting_label);

-- A reviewer app belongs to one development. Both app columns share that set.
-- The add command refuses a reuse. A unique index cannot cover both columns.

-- The check run name GitHub shows on the pull request head.
-- A development with no rows has no CI.
CREATE TABLE development_check (
    development_name TEXT NOT NULL REFERENCES development (name),
    name             TEXT NOT NULL,
    PRIMARY KEY (development_name, name)
);
