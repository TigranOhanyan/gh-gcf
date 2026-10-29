# Software Factory

A factory turns ready issues in app repos into reviewed pull requests, using Cursor agents for the judgment work and deterministic code for everything else.

## Repos

**Factory repo**:
A user's own repo that holds a factory's scheduled workflow and a separate config and state for each app repo it drives.
_Avoid_: host factory, control repo

**App repo**:
A repo whose issues a factory builds. It holds only its own code, CI and agent rules, and does not know the factory exists.
_Avoid_: host, host repo, target repo

**Bind**:
The commit on a factory repo's main branch that installs that factory.
_Avoid_: setup, init

## Running

**Tick**:
One scheduled run of the factory that looks at every app repo and takes the next action for each issue and pull request.
_Avoid_: reconcile, cycle, job

**Config**:
What the user wants the factory to do: its labels, Apps, blueprints, and the CI checks that can claim a fixer. Only the user changes it.
_Avoid_: settings, desired state

**State**:
What the factory has done and seen, including the config data kept by compilation. Only the tick changes it.
_Avoid_: database, history

**Compilation**:
The tick's first step. It updates State's builders, reviewers, and fixers from Config.
_Avoid_: migration, validation

## Agents

**Blueprint**:
A named setup in Config for one kind of agent: a builder, a reviewer, or a fixer.
_Avoid_: template, definition, spec

**Builder**:
A blueprint tied to a starting label and an optional GitHub App. Its agent implements a ready issue and opens the pull request.
_Avoid_: implementer, coder

**Reviewer**:
A blueprint tied to one builder. Its agent posts a verdict on that builder's pull request.
_Avoid_: code reviewer

**Standards reviewer**:
The reviewer that judges a pull request against the app repo's documented coding standards and a baseline of code smells.
_Avoid_: code reviewer, linter

**Spec reviewer**:
The reviewer that judges a pull request against the issue it claims to implement.
_Avoid_: requirements reviewer, QA

**Fixer**:
A blueprint tied to one builder. Its agent applies the change requests on that builder's pull request, once.
_Avoid_: builder (second run), repairer, review applier

**Agent**:
The Cursor agent started for a run.
_Avoid_: bot, builder

**Run**:
One blueprint's work on one issue.
_Avoid_: execution, building

**Verdict**:
A reviewer's decision on a pull request: approve or request changes. A plain comment is not a verdict.
_Avoid_: result, outcome

**Check**:
A named CI result on a pull request that can claim a fixer.
_Avoid_: workflow, action, job

**Attention**:
The mark that the factory has stopped on an issue and a person must look at it.
_Avoid_: failure, blocked
