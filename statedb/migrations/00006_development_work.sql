-- +goose Up
-- A development work points at the agents it has started.
-- A development agent does not point back at the work.
-- status is empty while the factory is still at work.

CREATE TABLE development_agent (
    id              TEXT PRIMARY KEY,
    role            TEXT NOT NULL CHECK (role IN ('builder', 'standards_reviewer', 'specification_reviewer', 'fixer')),
    cursor_agent_id TEXT,
    cursor_run_id   TEXT,
    started_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    finished_at     TEXT
);

CREATE TABLE development_work (
    id                           TEXT    PRIMARY KEY,
    development_name             TEXT    NOT NULL REFERENCES development (name),
    issue_number                 INTEGER NOT NULL UNIQUE,
    issue_url                    TEXT    NOT NULL,
    pull_request_number          INTEGER,
    builder_head_sha             TEXT,
    fixer_head_sha               TEXT,
    builder_agent                TEXT REFERENCES development_agent (id),
    standards_reviewer_agent     TEXT REFERENCES development_agent (id),
    specification_reviewer_agent TEXT REFERENCES development_agent (id),
    fixer_agent                  TEXT REFERENCES development_agent (id),
    status                       TEXT    NOT NULL DEFAULT ''
        CHECK (status IN ('', 'attention', 'finished'))
);

CREATE TABLE development_event (
    id         TEXT PRIMARY KEY CHECK (length(id) = 26),
    step_id    TEXT NOT NULL REFERENCES step (id),
    agent_id   TEXT NOT NULL REFERENCES development_agent (id),
    kind       TEXT NOT NULL CHECK (kind IN ('starting', 'ack', 'rejected', 'completed', 'failed')),
    message    TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
