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

## Setup

**Setup**:
The user's recording of an app repo's config.
_Avoid_: bind, init

**TUI**:
The interactive Setup. Its command is `setup tui`.
_Avoid_: wizard

**Screen**:
The one place the TUI is showing. A return builds it again.
_Avoid_: page, widget, step

**Repo menu**:
The labels, attached Apps, and developments of one app repo.
_Avoid_: sections

## Running

**Tick**:
One scheduled run of the factory that looks at every app repo and takes the next step for each issue and pull request.
_Avoid_: reconcile, cycle, job

**Step**:
One named piece of a tick, recorded in each app repo's state.
_Avoid_: phase, stage, job, action

**Event**:
A moment of one agent call: starting, ack, rejected, completed, or failed. Finding the agent still running writes none.
_Avoid_: log, status

**Executor**:
The part of the factory that carries one command across every app repo.
_Avoid_: engine, runner

**Config**:
What the user wants the factory to do: its labels, Apps, developments, and the GitHub checks that can claim a fixer. Only the user changes it.
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
A development's copy from Config into State.
_Avoid_: migration, validation

## Developments

**Development**:
The configured flow that takes a ready issue through a builder, a standards reviewer, a specification reviewer, and a fixer.
_Avoid_: member, blueprint, pipeline, coding flow

**Instruction**:
The text one role in a development gives its agent. Each of the four roles has its own.
_Avoid_: brief, prompt

**Builder**:
The role in a development whose agent implements a ready issue and opens the pull request.
_Avoid_: implementer, coder, member

**Reviewer**:
A standards reviewer or a specification reviewer. Each has its own App.
_Avoid_: code reviewer

**Standards reviewer**:
The reviewer that judges a pull request against the app repo's documented coding standards and a baseline of code smells.
_Avoid_: linter, convention reviewer

**Specification reviewer**:
The reviewer that judges a pull request against the issue it claims to implement.
_Avoid_: spec reviewer, requirements reviewer, QA

**Fixer**:
The role that applies a changes request or a red GitHub check on that pull request, once. It stays idle when both reviewers approved and every listed GitHub check passed.
_Avoid_: builder (second run), repairer, review applier

**Development work**:
One issue taken by one development. It points at the agents it has started.
_Avoid_: coding work, work, task, run, cycle

**Agent**:
One of a development's four roles acting through Cursor. The development work points at it, and the agent does not point back.
_Avoid_: bot, run

**Verdict**:
A reviewer's decision on a pull request: approve or request changes. A comment is not a verdict.
_Avoid_: result, outcome

**GitHub check**:
A named result GitHub attaches to a pull request. It belongs to one development and can claim that development's fixer.
_Avoid_: check, CI check, workflow, action, job

**Attention**:
The mark that the factory has stopped on an issue and a person must look at it. A development has one attention label.
_Avoid_: failure, blocked

**Finished**:
A development work the factory will not pick up again.
_Avoid_: merged, closed
