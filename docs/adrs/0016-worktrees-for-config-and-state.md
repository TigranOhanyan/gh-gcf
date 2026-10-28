# The tick opens config and state as worktrees

The tick fetches `main` and `state`, then opens `config.db` on a worktree of `main` and `state.db` on a worktree of `state`. The fetch is as fresh as reading those files from the GitHub API, and the state worktree is the file the tick commits and pushes. A test builds that repository in its own setup: it commits the `config.db` it just created on `main` and the `state.db` it just created on `state`. Each test's databases differ, so the repository is not checked in.

## Considered Options

- The GitHub contents API, with WireMock in tests: rejected, because the tick runs inside the factory repo, and a downloaded copy still has to be committed onto the fetched tip of `state`.
- A checked-in bare repository: rejected, because the databases are different in each test.
