# A label name is whatever the user stores

Config stores the label name the user wrote. The factory creates that name on the app repo and does not add a suffix or any other spelling of its own.

## Considered Options

- A required `-f` suffix, as in `todo-f` and `attention-f`: rejected, because each factory chooses its own label names.
