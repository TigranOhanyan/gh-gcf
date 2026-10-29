# Bind keeps worktrees inside the factory repo

Bind commits `.worktrees/.gitignore` and an empty `.worktrees/internal_use_only` on `main`. That gitignore ignores every other file in `.worktrees`, including a checkout such as `.worktrees/state`. The root `.gitignore` is left alone, and `copy` does not create or require these files.

## Considered Options

- A directory outside the factory repo: rejected, because the checkouts belong with the factory.
- A rule in the root `.gitignore`: rejected, because bind would edit a file at the repo root.
- `.git/info/exclude`: rejected, because that rule is local to one clone and is not in the bind commit.
