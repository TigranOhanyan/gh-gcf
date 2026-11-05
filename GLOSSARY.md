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
The commit that installs a factory in its factory repo.
_Avoid_: setup, init

**Setup**:
The user's recording of an app repo's config.
_Avoid_: bind, init

## Running

**Tick**:
One scheduled run of the factory that looks at every app repo and takes the next step for each issue and pull request.
_Avoid_: reconcile, cycle, job

**Step**:
One named piece of a tick, recorded in each app repo's state.
_Avoid_: phase, stage, job, action

**Event**:
A moment of one agent call: starting, ack, rejected, completed, or failed. A poll that finds the agent still running writes none.
_Avoid_: log, status

**Executor**:
The part of the factory that carries one command across every app repo.
_Avoid_: engine, runner

**Config**:
What the user wants the factory to do: its labels, Apps, members, and the CI checks that can claim a fixer. Only the user changes it.
_Avoid_: settings, desired state

**Label**:
A name the user records in an app repo's config. The factory creates that name on the app repo when it is missing.
_Avoid_: tag, marker

**App**:
A GitHub App a reviewer acts as. GitHub assigns its App ID, and that id identifies the app.
_Avoid_: bot, installation, slug, display name, factory app

**Factory app**:
The GitHub App the tick uses when it creates and reads GitHub resources.
_Avoid_: reviewer app, builder app, bot

**Installation**:
An App installed on the account that owns an app repo.
_Avoid_: app, token

**State**:
What the factory has done and seen. The tick and the steps write it. The user does not.
_Avoid_: database, history

**Compilation**:
One flow's copy of its members from Config into State. The coding flow's compilation is separate from a later flow's.
_Avoid_: migration, validation

## Agents

**Member**:
A builder, a reviewer, or a fixer named in an app repo's config.
_Avoid_: blueprint, participant

**Instruction**:
The text a member gives its agent, kept in a file named after that member.
_Avoid_: brief, prompt

**Builder**:
A member tied to a starting label. Its agent implements a ready issue and opens the pull request.
_Avoid_: implementer, coder

**Reviewer**:
A member tied to one builder and to its own App. Its agent posts a verdict on that builder's work.
_Avoid_: code reviewer

**Standards reviewer**:
The reviewer that judges a pull request against the app repo's documented coding standards and a baseline of code smells.
_Avoid_: code reviewer, linter

**Spec reviewer**:
The reviewer that judges a pull request against the issue it claims to implement.
_Avoid_: requirements reviewer, QA

**Fixer**:
The one member a builder has for applying change requests on that builder's pull request, once. A builder may have none.
_Avoid_: builder (second run), repairer, review applier

**Coding work**:
The factory's handling of one issue by one builder. Its agents point at it.
_Avoid_: work, team, task, run, cycle

**Agent**:
One member acting through Cursor on one coding work. A builder, a reviewer, and a fixer are different kinds.
_Avoid_: bot, run

**Verdict**:
A reviewer's decision on a pull request: approve or request changes. A plain comment is not a verdict.
_Avoid_: result, outcome

**Check**:
A named CI result recorded for one builder. It can claim that builder's fixer.
_Avoid_: workflow, action, job

**Attention**:
The mark that the factory has stopped on an issue and a person must look at it.
_Avoid_: failure, blocked
