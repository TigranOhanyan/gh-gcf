-- +goose Up
-- Compiled development. Label and App references are not enforced here.
-- The four instruction hashes are the files under developments/<name>/.

CREATE TABLE development (
    name                                      TEXT PRIMARY KEY,
    starting_label                            TEXT NOT NULL,
    attention_label                           TEXT NOT NULL,
    standards_reviewer_app                    INTEGER NOT NULL,
    specification_reviewer_app                INTEGER NOT NULL,
    builder_app                               INTEGER NOT NULL,
    fixer_app                                 INTEGER NOT NULL,
    builder_instruction_sha256                TEXT NOT NULL,
    standards_reviewer_instruction_sha256     TEXT NOT NULL,
    specification_reviewer_instruction_sha256 TEXT NOT NULL,
    fixer_instruction_sha256                  TEXT NOT NULL
);

CREATE TABLE development_check (
    development_name TEXT NOT NULL REFERENCES development (name),
    name             TEXT NOT NULL,
    PRIMARY KEY (development_name, name)
);
