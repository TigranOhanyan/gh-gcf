# gh gcf

`gh gcf` is a GitHub CLI extension. A factory takes ready issues in app repos and turns them into reviewed pull requests. Cursor agents do the judgment work. A development claims a ready issue and starts the agent that opens the pull request.

## Repos

**gcf** is this extension. Install it with `gh`. Your issues and pull requests do not live here.

**Factory repo** is a repository you create. It holds the config and state for each app repo, and the install of the tick.

**App repo** is a repository whose issues the factory builds. It holds its own code. It has no factory files.

## Install

```shell
gh extension install TigranOhanyan/gh-gcf
```

`gh extension upgrade gcf` installs a newer release.

## Run a factory

1. Create an empty repository and clone it. This is the factory repo.

2. Create the factory app. The steps are under [GitHub Apps](#github-apps). Install it on the factory repo and on each app repo before the tick runs.

3. In the factory checkout, on the branch you want to change:

```shell
gh gcf bind
```

Bind writes the deploy workflow, the delete workflow, the secrets workflow, `infrastructure.yaml`, `tick/Dockerfile`, and `secrets.yaml`, and commits. Push the branch. When the factory repo only accepts changes through a pull request, open one from this branch. A merge to `main` deploys the empty secrets. The tick reads `main` after that merge.

4. In the factory repo's Actions secrets, set `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`, and `AWS_APPLICATION_BUCKET`. In the factory repo's Actions variables, set `STACK_NAME`. Two factories in one AWS account use two names. After the secrets stack exists, set `cursor-api-key`, `factory-app-id`, and `factory-app-private-key` in Secrets Manager under the `<STACK_NAME>-secrets` prefix. A new value is picked up on the next tick.

5. Create a fine-grained personal access token that can read the contents of `TigranOhanyan/gh-gcf` and no other repository. In the factory repo's Actions secrets, set `GCF_RELEASE_TOKEN` to that token. Deploy downloads the pinned release with it.

6. From the factory checkout:

```shell
gh gcf setup tui
```

Enter opens Repos or Apps. `a` adds a repo, a label, or an App ID. An App is the number GitHub shows as the App ID. Setup writes that id as `apps/<app id>`, and a repo can then use it. Adding the App on GitHub, and storing its key, are under [GitHub Apps](#github-apps).

Inside a repo, setup records labels, the Apps that repo uses, and developments. A label is a name you choose for one app repo. The starting label marks an issue as ready. The attention label marks an issue the factory stopped on, so a person has to look. The factory creates a missing label on that app repo. A label recorded for one app repo is not a label on another.

A development has a name, a starting label, one attention label, two reviewer App IDs, a builder App ID, and a fixer App ID. The two reviewers do not share an App. The builder and the fixer may share an App, and neither of those is a reviewer App. Adding it creates `repos/<owner>/<name>/developments/<name>/` with empty `builder.md`, `standards_reviewer.md`, `specification_reviewer.md`, and `fixer.md`. When that folder is already there, the add stops and the folder should be removed. You edit those files in your editor. A GitHub check name on the development can claim the fixer.

Setup writes the config in the checkout you have open. Commit and push that change the same way as bind.

The same writes are commands under `setup`. A command takes every value in one invocation, writes the open checkout, and does not commit.

```shell
gh gcf setup repo add --owner octo --name app
gh gcf setup label add --owner octo --repo app --name ready --color ededed
gh gcf setup label list --owner octo --repo app
gh gcf setup apps add --id 42
gh gcf setup apps attach --owner octo --repo app --id 42
gh gcf setup development add --owner octo --repo app --name features --starting ready --attention stop --standards-app 42 --specification-app 43 --builder-app 44 --fixer-app 44
gh gcf setup development check add --owner octo --repo app --development features --name "ci / unit"
```

On the factory repo, run the Factory Deploy workflow after the secret values are set. It downloads the release pinned by bind, builds the tick image into ECR in that account, and deploys it. The tick runs every 5 minutes and reads the secrets on each launch. Factory Delete removes the tick stack, the secrets stack, and the tick ECR repository. `STACK_NAME` is the tick stack. The secrets stack is that name with `-secrets` appended. Adding an App takes another bind and a merge to `main`, then the new secret's value. Deleting the factory repo leaves the tick running.

From the factory checkout, after `gh gcf run tick` prints an id:

```shell
gh gcf compile app --tick <id>
gh gcf compile label --tick <id>
gh gcf compile development --tick <id>
gh gcf run builder --tick <id>
gh gcf check builder --tick <id>
gh gcf run standards-reviewer --tick <id>
gh gcf check standards-reviewer --tick <id>
gh gcf run specification-reviewer --tick <id>
gh gcf check specification-reviewer --tick <id>
gh gcf run fixer --tick <id>
gh gcf check fixer --tick <id>
```

## GitHub Apps

A person creates every App on GitHub. There are two kinds.

### Factory app

The factory app is the identity the tick uses when it creates and reads GitHub resources. It does not build, review, or fix.

1. Create the App and note the App ID GitHub shows.
2. Generate a private key. Store that PEM in the `factory-app-private-key` secret. Store the App ID in the `factory-app-id` secret.
3. Install it on the factory repo and on each app repo before the tick runs.

### Reviewer App

A reviewer App is the identity one reviewer posts as. A development has two, and they are different Apps. The builder and the fixer have none.

1. Create one App per reviewer and note each App ID.
2. Generate a private key for each. After `apps/<app id>` is on `main`, store that PEM in the `app/<app id>` secret.
3. Install each App on the account that owns the app repo. `compile app` records that installation. A missing or suspended installation stops the step.
4. Register and attach the id from the factory checkout:

```shell
gh gcf setup apps add --id 42
gh gcf setup apps attach --owner octo --repo app --id 42
```

5. Run `gh gcf bind` again after `apps/<app id>` exists, so the secrets template creates `app/<app id>`. Commit and push that change the same way as the first bind. The merge creates the empty secret. Store the PEM there. The next tick reads it.

## Steps

A tick runs these steps in this order. Each one takes the tick id printed by `gh gcf run tick`.

### `compile app`

`compile app` records the client id and the installation id for each App configured on a repo. The same installation is left as it is. A missing or suspended installation stops the step.

### `compile label`

`compile label` creates a configured label the app repo does not yet have.

### `compile development`

`compile development` copies each development into state, with its check names and the SHA-256 of `builder.md`, `standards_reviewer.md`, `specification_reviewer.md`, and `fixer.md`. A missing file stops compilation. An unchanged development is left as it is. A changed instruction stores the new hash.

### `run builder`

`run builder` claims an open issue that has the development's starting label and no development work. The work stores the issue number and URL and points at the builder. When the instruction hash differs from the compiled hash, that development waits. A refused create sets the work to attention and applies the attention label. A later run leaves that issue alone.

### `check builder`

`check builder` records a finished builder: the pull request number and the builder head SHA. A builder that is still running changes nothing. A finished builder with no pull request, or a failed run, sets attention and applies the attention label. A failed run records no finish time, so the standards reviewer does not start.

### `run standards-reviewer`

`run standards-reviewer` starts that reviewer after the builder has finished. It does not wait for checks and it does not read a verdict. A hash mismatch waits. A refused create sets attention.

### `check standards-reviewer`

`check standards-reviewer` records that the reviewer has finished. A run that is still going changes nothing. A failed run sets attention and applies the attention label.

### `run specification-reviewer`

`run specification-reviewer` starts that reviewer after the standards reviewer has finished. It does not wait for checks, and no verdict is read between the two reviewers. A hash mismatch waits. A refused create sets attention.

### `check specification-reviewer`

`check specification-reviewer` records that finish. A run that is still going changes nothing. A failed run sets attention and applies the attention label.

### `run fixer`

`run fixer` reads the listed checks on the builder head SHA once both reviewers have finished. The newest run of each listed name counts. A missing run, or a queued or in-progress run, waits. Success, skipped, and neutral have passed. Failure, cancelled, and timed out are red. A name that is not listed is ignored. No listed names means the checks have already passed.

Verdicts are read only after every listed check has finished, including a red one. A verdict is that App's latest `APPROVED` or `CHANGES_REQUESTED` review. A `COMMENTED` review is a missing verdict. A missing verdict sets attention and does not start the fixer. With both verdicts present, a changes request or a red check starts the fixer once. Two approvals and every listed check passed set the work to finished and leave the pull request as it is. The fixer stays unset.

### `check fixer`

`check fixer` records the fixer head SHA when that run finishes. A run still going changes nothing. A failed run sets attention and records no fixer head SHA. After the fixer has finished, the listed checks are read on the fixer head SHA and reviews are not read again. A check that has not finished waits. Every listed check passed sets the work to finished and leaves the pull request as it is. One red check sets attention. The fixer does not run again.

A later tick leaves an attention work and a finished work alone. The factory does not merge the pull request or close the issue.
