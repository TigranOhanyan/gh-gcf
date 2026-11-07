# A development is the configured flow

A development is the one config row for taking ready issues to pull requests. It has a name, a starting label, one attention label, the standards reviewer's App ID, and the specification reviewer's App ID. The two reviewers do not share an App (`0003-one-app-per-reviewer.md`). The builder, the standards reviewer, the specification reviewer, and the fixer are fixed roles. The row does not name them, and the fixer is always one of them. Check names belong to the development. Two developments may record the same name. An empty list means that development has no CI.

The instructions are `repos/<owner>/<name>/developments/<development>/builder.md`, `standards_reviewer.md`, `specification_reviewer.md`, and `fixer.md`. A missing file stops compilation. Compilation copies the development into state and stores the SHA-256 of each instruction. When the tick is about to start an agent and the file's hash differs, that agent waits. The next tick compiles again and stores the new hash.

Attention is one label on the development. The tick applies it to the issue whose development work stopped. One stopped issue does not stop the development.

This supersedes the member tables in `0030-members-and-agents-replace-blueprints-and-runs.md`, the per-member attention label in `0015-blueprint-names-the-attention-label.md`, and the builder-owned check list in `0029-a-check-belongs-to-one-builder.md`.

## Considered Options

- A blueprint row pointing at member rows: rejected, because the four roles are fixed and a second row would only repeat them.
- One shared App for both reviewers: rejected, because GitHub keeps one standing verdict per identity (`0003-one-app-per-reviewer.md`).
- An optional fixer: rejected, because the role is always part of the flow. A development work that never needs it never starts that agent.
- Naming the row a builder: rejected, because the row is the whole flow, and the builder is one role inside it.
