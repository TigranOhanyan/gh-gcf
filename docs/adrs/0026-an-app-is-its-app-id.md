# An App is its App ID

Config stores an App as the App ID GitHub assigned. That integer is the primary key, and a reviewer member references it (`0030-members-and-agents-replace-blueprints-and-runs.md`). State stores the same id, the client id, and the installation id. The `app` step fills the client id and the installation id from GitHub, and it writes the state row only after that response. GitHub's display name can be changed and is not stored. The user does not invent a name, and does not type the client id.

## Considered Options

- A name the user invents: rejected, because the App already has an id, and a second name can drift from it.
- GitHub's display name as the key: rejected, because a rename would look like the App had disappeared.
- The user types the client id as well: rejected, because the installation response already carries it.
