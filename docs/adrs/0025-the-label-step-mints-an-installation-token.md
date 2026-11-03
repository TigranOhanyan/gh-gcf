# The label step mints an installation token

When `FACTORY_GITHUB_APP_ID` and `FACTORY_GITHUB_APP_PRIVATE_KEY` are set, the label step signs a JWT for that app and asks GitHub for the installation on the app repo, then for an installation token. The get and the create send that token. Each app repo is exchanged on its own, because each owner can have a different installation. The private key stays in the environment and is not written into an event. When both variables are empty, the step keeps the user token so a person can run it from a checkout. The workflow passes the two secrets and leaves `GH_TOKEN` as the Actions token that installs `gh-gcf` and pushes state.

## Considered Options

- Minting one token in the workflow and exporting it as `GH_TOKEN`: rejected, because one installation does not cover every app repo, and that export would replace the Actions token the job uses to push state.
