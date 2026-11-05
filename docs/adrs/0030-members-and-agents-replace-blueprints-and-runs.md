# Members and agents replace blueprints and runs

A member is the shared config row: a name, a role, and an attention label. `builder_member`, `reviewer_member`, and `fixer_member` reference that row by name. Each child fixes its own role. The builder member holds the starting label. The reviewer member holds that builder and the App. The fixer member holds that builder, and a builder has at most one. The instruction is the file `repos/<owner>/<name>/members/<name>.md`. The row does not store the path.

A work is one issue taken by one builder. It holds the pull request number and head SHA once that builder opens them. An agent belongs to one work. Its id is `bc-` plus a UUID v7 sent to Cursor, and the child row uses that same id. The create response's run id is stored on the agent once that response arrives. `builder_agent`, `reviewer_agent`, and `fixer_agent` extend it by that id. The reviewer agent stores the verdict. There is no run table. Every agent points at the work, so a fixer with no reviewers still has a place to hang.

This supersedes the blueprint and run nouns in `0006-blueprint-per-kind.md`, the pull-request columns on the builder run in `0012-fixer-starts-when-checks-and-reviewers-finish.md`, and the blueprint row in `0015-blueprint-names-the-attention-label.md`.

## Considered Options

- One run table with a column for each kind: rejected, because a new kind would add columns the other agents do not use.
- Pointing a reviewer agent and a fixer agent at the builder agent: rejected, because the work is the group.
- The word blueprint: rejected, because it does not say what the row is.
