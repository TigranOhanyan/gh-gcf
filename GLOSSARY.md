# Software Factory

A factory turns ready issues in app repos into reviewed pull requests, using Cursor agents for the judgment work and deterministic code for everything else.

## Repos

**Factory repo**:
A user's own repo that holds a factory's config, state and scheduled workflow, and drives one or more app repos.
_Avoid_: host factory, control repo

**App repo**:
A repo whose issues a factory builds. It holds only its own code, CI and agent rules, and does not know the factory exists.
_Avoid_: host, host repo, target repo

## Running

**Tick**:
One scheduled run of the factory that looks at every app repo and takes the next action for each issue and pull request.
_Avoid_: reconcile, cycle, job

**Config**:
What the user wants the factory to do: its app repos, Apps, required checks, models and limits. Only the user changes it.
_Avoid_: settings, desired state

**State**:
What the factory has done and seen: the agents it started and how they ended. Only the tick changes it.
_Avoid_: database, history

## Agents

**Builder**:
The agent that implements a ready issue and opens its pull request.
_Avoid_: implementer, coder

**Standards reviewer**:
The agent that reviews a pull request against the app repo's documented coding standards and a baseline of code smells.
_Avoid_: code reviewer, linter

**Spec reviewer**:
The agent that reviews a pull request against the issue it claims to implement.
_Avoid_: requirements reviewer, QA

**Fixer**:
A fresh agent that applies the change requests and fixes the failing checks on a builder's pull request, once.
_Avoid_: builder (second run), repairer

**Verdict**:
A reviewer's decision on a pull request: approve or request changes. A plain comment is not a verdict.
_Avoid_: result, outcome

**Attention**:
The mark that the factory has stopped on an issue and a person must look at it.
_Avoid_: failure, blocked