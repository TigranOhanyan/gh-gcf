# What Cursor allows an agent

Cursor's cloud agent is one GitHub identity, the Cursor app, and a run of that agent cannot be handed both an installation token and an id the factory chose. The factory still wants each role to act as its own App. The runs below are why the factory sends the token, lets Cursor choose the id, and tells the agent to use `gh`.

## What Cursor constrains

A create accepts `envVars`. `gh` reads `GH_TOKEN`, then `GITHUB_TOKEN`, from the process environment, and either one overrides credentials saved in `gh`. The same create refuses a body that also has a client `agentId`. The error is `envVars cannot be combined with a client-supplied agentId; omit agentId so the server mints one.` The field is in beta and can be ignored without a failed status, so a first run has to show that the values arrived.

The run token Cursor mints for the agent does not include Issues. Staff called that intentional. `gh issue view` returns 403. A public issue can still be fetched with no credentials. A private issue cannot. Granting Issues on the Cursor app's install page does not widen the run token. Staff's workaround is a personal access token named `GH_TOKEN` or `GITHUB_TOKEN`.

`gh` is described to the model as read-only, and pull-request writes are supposed to go through Cursor's own tool. That tool is not `gh`. It posts as `cursor[bot]`. On a public pull request it wrote a comment and GitHub rejected `addPullRequestReview` for that integration, so no `APPROVED` or `CHANGES_REQUESTED` review was stored. A later run replied to a comment on the same pull request after the repo was made private, so a private pull request is visible once the Cursor app is installed. A comment is still not a verdict. GitHub dismisses a stale approval only for a formal approving review, and only when the repository is set to do so.

Git push and pull use a rewrite Cursor installs, `x-access-token` embedded in the GitHub URL. `GH_TOKEN` does not replace it. Cursor puts the rewrite back on a later turn.

An installation token is minted from the App id and the private key. The key never goes to the agent. The token lasts about an hour and acts as the App. GitHub keeps one standing verdict per identity.

## What we tried

`POST /v1/agents` with a raw key after the word `Basic` returns 401 `Invalid User API Key`. The same key as Bearer, or as Basic of `key:`, returns 200 from `GET /v1/models`.

A create that sends both `agentId` and `envVars` returns 400 with the sentence above.

`GET /v1/agents/{id}/runs/{runId}` on a finished run returns `result`, the assistant's final text. The check step used to drop it.

With an installation token for the standards App in `GH_TOKEN` and `GITHUB_TOKEN`, and a prompt that told the agent to use `gh` anyway, the agent commented `bonjour` on issue 3. The comment is from `gcf-standards[bot]`, not `cursor[bot]`.

A second run set `GH_PERMISSION` to `WRITE_READ` and told the agent that `gh` is read-only unless that value is set. The agent submitted `gh pr review` on pull request 4. The review is `CHANGES_REQUESTED`, body `bonjour`, from `gcf-standards[bot]`. `GH_PERMISSION` is not read by `gh`. It is there so the instruction is one rule with an exception the model can check, instead of two sentences that disagree.

Both runs also left a Cursor branch behind. The factory does not use that branch, except the builder check in `0043-the-builder-pull-request-is-cursors-then-the-branchs.md`, which uses it as the head when the run has no pull request URL.

## What we rejected

Parsing a verdict out of `result` or out of a comment. The prose can say "request changes" when GitHub stored no review. The factory would also have to notice a new commit and throw the verdict away. GitHub already does that for a formal approval.

A personal access token for the builder and the fixer. Reads and writes share one `gh` token, so checkbox edits and comments would be that person.

The factory app's token for those two roles. That mixes the tick's identity with the agents.

One column shared by the builder and the fixer. The Apps may differ later. The columns are not counts. How often the fixer ran, and why the builder did not finish in one shot, are already on the work.

A `gh` extension, or a separate program the agent installs, as the only way past the read-only line. The review run showed `gh` itself is enough. Playwright holding a person's GitHub session, in order to click Create App, is a more powerful secret than one private key, and GitHub still asks that person to approve.

Repeating a create whose response was lost, and setting attention an hour later. Both stay deferred. A lost create stays lost.

## What we decided

Every role receives its App's installation token as `GH_TOKEN` and `GITHUB_TOKEN`, and `GH_PERMISSION=WRITE_READ`. The create does not send `agentId`. Cursor's id is stored when the response arrives. The factory used to choose a `bc-` id and send it, as `0031-coding-does-not-repeat-a-create.md` describes, so the id was known before Cursor answered and a repeat of that create conflicted instead of starting a second agent. The token gives that up. A lost response now leaves an agent the factory cannot name. The no-repeat stands. The factory appends a trailer to the instruction and does not write it back. The reviewer's trailer requires `gh pr review --approve` or `gh pr review --request-changes`. The builder and fixer trailer does not. The agent reads the issue with `gh`. The factory does not paste the issue text.

The development stores `builder_app` and `fixer_app`. Both are required. They may be equal. Each differs from both reviewer Apps. Neither is the factory app. The person creates those Apps. A manifest can pre-fill the permissions and hand back the private key. It cannot skip the owner's click, and it does not install the App.

## Later

The two coding Apps may diverge. A manifest can shorten the form. The owner's approval stays. Attention an hour after a lost create stays deferred. If Cursor one day accepts `envVars` together with an id the factory chose, the factory can send its id again. If Cursor's run token gains Issues and pull-request reviews, the coding Apps are no longer required for the read. A verdict still needs an identity that is not `cursor[bot]`.
