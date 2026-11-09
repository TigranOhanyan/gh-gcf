# The open config database stops at the repo menu

The repo menu holds no `config.db`. Entering labels or attached apps from it opens that file and passes the handle to each screen built from there. The screen that builds the repo menu again closes the handle first. A missing file leaves the repo menu showing and says the config is missing. Adding a repo creates a database, closes it, and builds the repo list. An App registered on the factory is a file (`0027-registered-apps-are-files.md`) and does not take a repo database.

## Considered Options

- Each screen opens and closes the database: rejected, because the list and the form that edits it would be two connections for one edit.
- The repo menu holds the handle for the whole visit inside the repo: rejected, because the menu does not read the database.
- A session value holds the handle across screens: rejected, because that value would outlive the screen that uses it.
