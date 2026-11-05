# CI check names live in config

The name of the CI check that can claim a fixer differs per app repo, so each repo's config lists those names. This supersedes `0010-ci-is-part-of-every-builder-flow.md`. A builder with no reviewers claims the fixer only from the checks named there.

The repo-wide list is superseded by `0029-a-check-belongs-to-one-builder.md`. Each builder has its own check names.

## Considered Options

- Watching every check on the pull request: rejected, because a repo's check names are not the same everywhere, and unnamed checks would claim a fixer by accident.
