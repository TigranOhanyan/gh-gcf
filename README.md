# gh gcf

`gh gcf` is a GitHub CLI extension. A factory takes ready issues in app repos and turns them into reviewed pull requests. Cursor agents do the judgment work. The factory claims the issue, starts the agent, collects the reviews, and applies one fix.

## Repos

**gcf** is this extension. Install it with `gh`. Your issues and pull requests do not live here.

**Factory repo** is a repository you create. It holds the scheduled workflow, and the config and state for each app repo.

**App repo** is a repository whose issues the factory builds. It holds its own code. It has no factory files.

## Install

```shell
gh extension install TigranOhanyan/gh-gcf
```

`gh extension upgrade gcf` installs a newer release.

## Run a factory

1. Create an empty repository and clone it. This is the factory repo.

2. Create a GitHub App. This is the factory app. The factory acts on GitHub as this app. Install it on the factory repo and on each app repo, before the tick workflow runs.

3. In the factory checkout, on the branch you want to change:

```shell
gh gcf bind
```

Bind writes the tick workflow and commits. Push the branch. When the factory repo only accepts changes through a pull request, open one from this branch. The tick reads `main` after that pull request is merged.

4. In the factory repo's Actions secrets, set `CURSOR_API_KEY`, `FACTORY_GITHUB_APP_ID`, and `FACTORY_GITHUB_APP_PRIVATE_KEY`. The private key is the PEM for the factory app.

5. From the factory checkout:

```shell
gh gcf setup
```

`a` adds an app repo. Enter opens it. `a` again adds a label for that repo.

A label is a name you choose for one app repo. The starting label marks an issue as ready. The attention label marks an issue the factory stopped on, so a person has to look. The factory creates a missing label on that app repo. A label recorded for one app repo is not a label on another.

Setup writes the config in the checkout you have open. Commit and push that change the same way as bind.
