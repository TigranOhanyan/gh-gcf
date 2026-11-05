-- +goose Up
-- Instruction fingerprint lives on the state member. Config does not store it.
ALTER TABLE member ADD COLUMN instruction_sha256 TEXT NOT NULL DEFAULT '';

CREATE TABLE issue (
    number INTEGER PRIMARY KEY,
    url    TEXT    NOT NULL
);

-- One issue is taken once. The pull request number arrives when the builder opens one.
CREATE TABLE coding_work (
    id                  TEXT    PRIMARY KEY,
    issue_number        INTEGER NOT NULL UNIQUE REFERENCES issue (number),
    builder_name        TEXT    NOT NULL REFERENCES builder_member (name),
    pull_request_number INTEGER,
    head_sha            TEXT
);

-- id is bc- plus a UUID v7. cursor_run_id arrives with the create response.
CREATE TABLE agent (
    id            TEXT PRIMARY KEY,
    role          TEXT NOT NULL CHECK (role IN ('builder', 'reviewer', 'fixer')),
    work_id       TEXT NOT NULL REFERENCES coding_work (id),
    member_name   TEXT NOT NULL,
    cursor_run_id TEXT,
    FOREIGN KEY (member_name, role) REFERENCES member (name, role),
    UNIQUE (work_id, member_name)
);

CREATE UNIQUE INDEX agent_one_builder_per_coding_work
    ON agent (work_id)
    WHERE role = 'builder';

CREATE UNIQUE INDEX agent_one_fixer_per_coding_work
    ON agent (work_id)
    WHERE role = 'fixer';

CREATE TABLE builder_agent (
    id   TEXT PRIMARY KEY REFERENCES agent (id),
    role TEXT NOT NULL DEFAULT 'builder' CHECK (role = 'builder')
);

CREATE TABLE reviewer_agent (
    id      TEXT PRIMARY KEY REFERENCES agent (id),
    role    TEXT NOT NULL DEFAULT 'reviewer' CHECK (role = 'reviewer'),
    verdict TEXT CHECK (verdict IS NULL OR verdict IN ('approve', 'changes_requested'))
);

CREATE TABLE fixer_agent (
    id   TEXT PRIMARY KEY REFERENCES agent (id),
    role TEXT NOT NULL DEFAULT 'fixer' CHECK (role = 'fixer')
);

-- Label rows are not events. The series belongs to one agent.
DROP TABLE event;

CREATE TABLE event (
    id         TEXT PRIMARY KEY CHECK (length(id) = 26),
    step_id    TEXT NOT NULL REFERENCES step (id),
    agent_id   TEXT NOT NULL REFERENCES agent (id),
    kind       TEXT NOT NULL CHECK (kind IN ('starting', 'ack', 'rejected', 'completed', 'failed')),
    message    TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
