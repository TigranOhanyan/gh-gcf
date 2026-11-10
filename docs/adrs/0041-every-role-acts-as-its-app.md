# Every role acts as its App

A builder and a fixer each have an App, and those two Apps may be the same one. Neither is a reviewer App, and neither is the factory app. Each role's agent receives that App's installation token. Cursor assigns the agent's id, because a token and an id the factory chooses cannot be sent together. A lost create stays lost. This supersedes the sentence in `0028-factory-app-does-not-build-or-fix.md` that gave a builder and a fixer no App. The constraints, the runs, and the later hopes are in `0042-what-cursor-allows-an-agent.md`.

## Considered Options

- The factory app's token for the builder and the fixer: rejected, because that mixes the tick's identity with the agents.
- A personal token: rejected, because issue edits and comments would be authored by that person.
- One shared column for the builder and the fixer: rejected, because the Apps may later differ.
- Parsing a verdict out of a comment: rejected, because a comment is not a verdict and GitHub does not dismiss it when a new commit is pushed.
