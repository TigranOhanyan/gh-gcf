# The factory app does not build or fix

A builder and a fixer have no GitHub App. The factory app is the identity the tick uses when it creates and reads GitHub resources. A reviewer keeps its own App, and two reviewers still do not share one. This supersedes the sentence in `0003-one-app-per-reviewer.md` that let the factory app build or fix.

## Considered Options

- An optional App column on the builder and the fixer, left empty: rejected, because a column with no use is a second way to say that those members have no App (`0030-members-and-agents-replace-blueprints-and-runs.md`).
