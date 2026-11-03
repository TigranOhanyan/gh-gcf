# Setup records config in the checkout that is open

Setup is the user recording an app repo's config. It uses the current directory as the factory repo and fails when that directory is not a git checkout. It writes config on the branch checked out there, applies any config migrations the file is missing, and does not commit, push, or create GitHub labels. The user commits and reviews that database in a pull request. The tick still reads config from main.

## Considered Options

- A factory path flag: rejected, because the user runs setup in the checkout they mean to change.
- Refusing every branch except main: rejected, because the change is reviewed before it reaches main.
- A text file beside the database: rejected, because a reviewer pulls the branch and opens the database.
- Creating the GitHub label from setup: rejected, because the label step creates a missing label and records it in state.
