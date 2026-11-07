# A check belongs to one builder

Superseded by `0034-a-development-is-the-flow.md`. A check name belongs to one development. How a run is chosen is in `0036-checks-then-verdicts-then-one-fixer.md`.

A check stores the GitHub check run name for one builder member (`0030-members-and-agents-replace-blueprints-and-runs.md`). Two builders in one repo may record the same name. An empty list means that builder has no CI. The table is `github_check`. This supersedes the repo-wide list in `0011-ci-check-names-live-in-config.md`. A check that is not listed still does not count.

## Considered Options

- One check list for the whole repo: rejected, because two builders in one repo can wait on different pipelines.
