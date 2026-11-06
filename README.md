# gh gcf

`gh gcf` is a GitHub CLI extension. A factory takes ready issues in app repos and turns them into reviewed pull requests. Cursor agents do the judgment work. The coding flow claims a ready issue and starts the agent that opens the pull request.

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

Inside a repo, setup records labels, the Apps that repo uses, and builders. A label is a name you choose for one app repo. The starting label marks an issue as ready. The attention label marks an issue the factory stopped on, so a person has to look. The factory creates a missing label on that app repo. A label recorded for one app repo is not a label on another.

A builder has a name, a starting label, and a different attention label. One starting label belongs to one builder. Its instruction is the file `repos/<owner>/<name>/members/<name>.md`. You edit that file in your editor. A reviewer belongs to one builder and needs its own App. Two reviewers do not share an App. A builder has at most one fixer. Each of those members has its own instruction file, named the same way.

Setup writes the config in the checkout you have open. Commit and push that change the same way as bind.

The same writes are commands under `setup`. A command takes every value in one invocation, writes the open checkout, and does not commit.

```shell
gh gcf setup repo add --owner octo --name app
gh gcf setup label add --owner octo --repo app --name ready --color ededed
gh gcf setup label list --owner octo --repo app
gh gcf setup apps add --id 42
gh gcf setup apps attach --owner octo --repo app --id 42
gh gcf setup member add-builder --owner octo --repo app --name features --starting ready --attention stop
gh gcf setup check add --owner octo --repo app --builder features --name "ci / unit"
```

The scheduled workflow records a tick, then runs `app`, `label`, `compile coding`, and `coding`, then pushes state. `app` checks each App ID configured for a repo. When that App is installed there, it records the client id and the installation id. A missing or suspended installation stops the step. `label` creates a configured label the app repo does not yet have.

From the factory checkout, after `gh gcf tick` prints an id:

```shell
gh gcf app --tick <id>
gh gcf label --tick <id>
gh gcf compile coding --tick <id>
gh gcf coding --tick <id>
```

## Coding

`compile coding` copies the coding flow into that repo's state: each builder, its reviewers, its fixer, and its checks. It stores the SHA-256 of each instruction file, including an empty file. A missing instruction file stops the step.

`coding` reads those builders in name order. It hashes each instruction file again. When the hash differs from the one compile stored, that builder waits for the next tick, and the other builders still start. When the hashes match, it looks for open issues that carry the builder's starting label.

An issue is left alone when a coding work already has that issue number, or when a pull request references the issue. Otherwise coding writes the issue, the coding work, and the agent, then calls Cursor. The prompt is the instruction file, then the issue number, title, and body. Cursor is asked to open the pull request. The agent's id is `bc-` plus a UUID v7, and that id is what Cursor stores.

A successful create records an ack and Cursor's run id. A failed create records the response body and adds the builder's attention label to the issue. The coding work stays. A later tick does not call Cursor again for that agent.
