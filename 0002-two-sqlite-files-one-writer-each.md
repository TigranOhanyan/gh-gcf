# Config and state are two SQLite files, each with one writer

`config.db` on `main` is written only by the user, and `state.db` on the `state` branch is written only by the tick. Git cannot merge binary files, so a single shared file would let a user's push silently erase rows the tick recorded meanwhile, such as a running agent. SQLite was chosen over TOML or JSON for config because it is queryable and shares migrations with state.

## Considered Options

- One SQLite file: rejected, because two writers lose updates.
- TOML or JSON config compiled into SQLite: rejected, because it adds a format and a sync step for no gain once each file has one writer.