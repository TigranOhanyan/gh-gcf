# CI is part of every builder flow

Superseded by `0011-ci-check-names-live-in-config.md`. Check names differ per app repo, so they belong in that repo's config.

Required checks are not listed in config. Every builder flow watches the pull request's CI. Reviewers are the config that varies the flow. A builder with no reviewers claims the fixer only when CI fails.

## Considered Options

- A required-check table in `config.db`: rejected, because CI belongs to the building flow, and an empty reviewer list already says that only CI can trigger the fixer.
