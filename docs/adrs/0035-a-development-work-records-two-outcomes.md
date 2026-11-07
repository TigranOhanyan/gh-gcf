# A development work records attention or finished

A development work is one issue taken by one development. It holds the issue number and URL. The pull request number and the builder's head SHA arrive when the builder opens them. The fixer's head SHA arrives only when a fixer has pushed. Status is empty while the factory is still at work, `attention` when a person must look, and `finished` when the factory leaves the pull request as it is.

One agent table holds every role: builder, standards reviewer, specification reviewer, and fixer. The work points at each agent it has started. The agent row has no work column. An empty fixer link means that fixer has not run. The agent records when the tick started it and when the tick discovered it had finished. That discovery is written in the same transaction as the agent's completed event. The event series stays the history. Verdicts are not copied onto the work or the agent. GitHub holds them.

This supersedes the per-role agent tables in `0030-members-and-agents-replace-blueprints-and-runs.md` and the phase status in `0033-a-coding-work-records-its-status.md`.

## Considered Options

- A status per phase, from building through fixed: rejected, because the agent links already say who has started and who has finished, and `reviewing` could not tell the two reviewers apart.
- Copying a verdict onto the work when that reviewer finishes: rejected, because Cursor does not send it, and the tick reads it only after the checks have finished.
- Four agent subtables: rejected, because every role has the same fields.
- An issue table beside the work: rejected, because GitHub is the source, and the work keeps the number and the URL.
