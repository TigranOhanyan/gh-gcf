# Copy refuses a config database that is behind the binary

`copy` returns an error when `config.db` is missing a goose migration that this binary contains. It does not migrate that file, and it does not write state. Migrating config is a separate command. `state.db` is the file `copy` commits, so opening it applies its migrations and the state commit includes them.

## Considered Options

- Apply the config migration in `copy` and commit `config.db`: rejected, because `copy` would commit the branch only the user writes.
- Apply the config migration and leave it uncommitted: rejected, because the edit would be implicit.
- Refuse when `config.db` is newer than the binary: postponed. See issue 16. Until then, `copy` does not check that case.
