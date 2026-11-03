# Each app repo has its own folder

The factory repo keeps one folder per app repo at `repos/<owner>/<name>/`. Those two path segments are the repo. `main` carries that folder's `config.db`. The `state` branch carries that folder's `state.db` only. The tick reads config from `main`. A copy of `config.db` on the `state` branch would go stale, and opening it would migrate that copy before the tick pushed it. A repo's blueprints live in its own config.

## Considered Options

- One `config.db` and one `state.db` for the whole factory: rejected, because each app repo has its own blueprints and its own runs.
- Storing owner and name in the database as well as the path: rejected, because the two folder names are already the repo.
