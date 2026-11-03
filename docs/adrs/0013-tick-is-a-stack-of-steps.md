# The tick is a stack of steps

The workflow runs compilation, then validates GitHub Apps, then runs the cycle steps. Each step reads Config, State, and GitHub, and can be run on its own. Compilation updates State's builders, reviewers, and fixers from Config. The database enforces foreign keys, so those references are not validated in application code. Validating Apps is the one check outside the database: an App in Config must exist, and State gains it when it does. Code cannot create or delete a GitHub App. A State row that Config no longer has is an error; deletion is unsupported (`0014-deletion-is-unsupported.md`).

## Considered Options

- One tick function that owns every decision: rejected, because each step has to be runnable alone and a failure in one must not half-write the next.
- A compilation that only validates and writes nothing: rejected, because State's blueprint rows have to exist before a run can reference them.
