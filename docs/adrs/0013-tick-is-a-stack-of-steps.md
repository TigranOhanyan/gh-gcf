# The tick is a stack of steps

The workflow runs compilation, then `app`, then labels, then the cycle steps. Each step reads Config, State, and GitHub, and can be run on its own. Compilation updates State's builders, reviewers, and fixers from Config. The database enforces foreign keys, so those references are not validated in application code. `app` is the check outside the database. For each App ID in that repo's config it asks GitHub for the installation. State gains the row, with the client id and the installation id, only when GitHub returns a usable installation and state does not yet have the App. A matching installation id is left alone. A different installation id updates the row. A missing or suspended installation is an error: the step writes no new row, and a row already there stays. Code cannot create or delete a GitHub App. An App that state has and config lacks is left in place, the same way a label is (`0014-deletion-is-unsupported.md`).

## Considered Options

- One tick function that owns every decision: rejected, because each step has to be runnable alone and a failure in one must not half-write the next.
- A compilation that only validates and writes nothing: rejected, because State's blueprint rows have to exist before a run can reference them.
- Throwing when state has an App that config lacks: rejected, because that cleanup is the later deletion flow, and the label step already leaves such a row.
