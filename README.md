# gh gcf

`gh gcf` is a GitHub CLI extension. A factory takes ready issues in app repos and turns them into reviewed pull requests. Cursor agents do the judgment work. A development claims a ready issue and starts the agent that opens the pull request.

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

Enter opens Repos or Apps. `a` adds a repo, a label, or an App ID. An App is the number GitHub shows as the App ID. Setup writes that id as `apps/<app id>`, and a repo can then use it. The private key is the Actions secret `GITHUB_APP_<app id>`. Bind writes that secret into the tick workflow.

Inside a repo, setup records labels, the Apps that repo uses, and developments. A label is a name you choose for one app repo. The starting label marks an issue as ready. The attention label marks an issue the factory stopped on, so a person has to look. The factory creates a missing label on that app repo. A label recorded for one app repo is not a label on another.

A development has a name, a starting label, one attention label, and two reviewer App IDs. The two reviewers do not share an App. Its instructions are `repos/<owner>/<name>/developments/<name>/builder.md`, `standards_reviewer.md`, `specification_reviewer.md`, and `fixer.md`. You edit those files in your editor. A GitHub check name on the development can claim the fixer.

Setup writes the config in the checkout you have open. Commit and push that change the same way as bind.

The same writes are commands under `setup`. A command takes every value in one invocation, writes the open checkout, and does not commit.

```shell
gh gcf setup repo add --owner octo --name app
gh gcf setup label add --owner octo --repo app --name ready --color ededed
gh gcf setup label list --owner octo --repo app
gh gcf setup apps add --id 42
gh gcf setup apps attach --owner octo --repo app --id 42
gh gcf setup development add --owner octo --repo app --name features --starting ready --attention stop --standards-app 42 --specification-app 43
gh gcf setup development check add --owner octo --repo app --development features --name "ci / unit"
```

The scheduled workflow records a tick, then runs `app`, `label`, `compile development`, the `run` commands that start agents, and the `check` commands that record a finished agent, then pushes state. `app` checks each App ID configured for a repo. When that App is installed there, it records the client id and the installation id. A missing or suspended installation stops the step. `label` creates a configured label the app repo does not yet have.

From the factory checkout, after `gh gcf run tick` prints an id:

```shell
gh gcf app --tick <id>
gh gcf label --tick <id>
gh gcf compile development --tick <id>
gh gcf run builder --tick <id>
gh gcf check builder --tick <id>
gh gcf run standards-reviewer --tick <id>
gh gcf run specification-reviewer --tick <id>
gh gcf run fixer --tick <id>
gh gcf check fixer --tick <id>
```

## Run

`run builder` starts the builder for an open issue that carries a development's starting label and has no development work. The work stores the issue number and URL and points at the development and the builder. The agent does not point back at the work. When the builder instruction no longer matches the compiled hash, that development waits and nothing is written. A refused create sets the work to attention and applies the development's attention label. A later run does not start another builder for that issue.
