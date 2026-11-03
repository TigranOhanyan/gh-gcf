# Config and state have separate schemas

`config.db` and `state.db` keep different schemas and different SQL migrations. They do not share a schema or a migration version. Opening either file applies that file's goose migrations from the binary before any work. Compilation is a separate step: it reflects config data into state inside one transaction. If a validation fails, the transaction is rolled back and the state branch is left unpushed. A successful compilation is pushed to the state branch. State changes that do not come from config are made by a separate command, which also pushes.

## Considered Options

- Shared migrations across both files: rejected, because the files hold different data and change for different reasons. The user changes config. The tick changes state.
