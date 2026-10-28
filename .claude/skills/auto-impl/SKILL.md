---
name: auto-impl
description: >-
  Build one agreed ticket with nobody to answer questions. A builder
  subagent works its acceptance criteria test-first, every gate is settled
  by rule, and the result is handed over as a draft pull request.
disable-model-invocation: true
---

# Implement autonomously

Build the GitHub issue the prompt names, with nobody there to answer. Invoking this skill is the agreement to build it: the ticket's acceptance criteria and the spec's contracts are settled. Do not propose first, and do not ask questions. Every decision is settled by a rule below.

You are the orchestrator. The builder subagent writes the tests and the code. You check its work, run the review, commit, and open the pull request. Run this skill in the main chat, so the review can start its own subagents.

Nobody reads this chat. Everything the user must see goes on the issue or the pull request.

## Short circuit

When something makes progress impossible, and no rule below settles it, stop. Do not guess past it.

Commit what is done following the `git-commit` skill, push the branch, and open the draft pull request as in step 6. Then comment on the pull request with the question that blocks the work: what you tried, what you found, and the decision or fact you need.

When nothing is written yet, there is nothing to commit. Comment the question on the issue instead.

## 1. Claim the ticket

Read `docs/agents/issue-tracker.md`. Remove the `ready-for-agent` label from the issue, so no second agent takes it.

Work on the current branch. When it is `trunk`, create a branch named `agent/<issue number>-<slug>` first. Never commit to `trunk`.

## 2. Check the inputs

Read the ticket and the spec. The spec is the issue in the ticket's "Parent" section. When there is none, the ticket is its own spec. Read `docs/agents/domain.md` and follow it: `GLOSSARY.md`, and the ADRs in `docs/adr/` that touch the area.

- The ticket: its acceptance criteria are the slices to build.
- The spec: its contracts (signatures, tables, API) and the seams where tests go.

When the ticket has no acceptance criteria, or the spec has no contract for what a criterion needs, comment on the issue with what is missing, and stop. Do not guess a contract.

## 3. Dispatch the builder

Start one builder subagent in the foreground. Give it the brief in `docs/agents/builder-brief.md`, with the paths filled in. Pass pointers, not copies: the subagent reads the ticket and the spec itself. Add this to the brief:

> When a criterion cannot be tested by code, build it without a test and leave it unticked. Report why code cannot test it, and the steps a person takes to test it by hand.

Settle what the builder reports:

- A criterion needs a contract change: short circuit. The question is the criterion and the contract it needs.
- A test cannot go red on its assertion: leave the criterion unticked, and dispatch a fresh builder for the criteria after it. List the criterion under "Stopped on".
- A criterion cannot be tested by code: comment on the issue with the criterion, why code cannot test it, what was built, and the steps to test it by hand, so a person can review and test it.

## 4. Check the work

Read the diff yourself. Every criterion has a test named after it, except one that cannot be tested by code. Run the package's tests. Then run the full suite with `make test-run`.

When a check fails, dispatch a fresh builder with the brief, the failure, and the report, at most twice. When it still fails, skip the review and hand over, with the failure in "Last test run".

## 5. Review

Mark the files the builder created with `git add -N <path>`, so the review sees them. Then follow `.claude/skills/code-review/SKILL.md`. The fixed point is `origin/trunk`, and the spec is the ticket and its parent issue, so the review has nothing to ask. The review runs in its own subagents, never in this chat.

Settle the findings by rule:

- Apply a breach of a documented standard.
- Apply a requirement that is missing or partial, and one that is implemented wrong.
- Do not apply a baseline smell, or behaviour the spec did not ask for. List them in the pull request.

Dispatch a fresh builder with the brief and the findings to apply. Check the work again, as in step 4, then review again. Stop after two rounds of fixes. What is still open goes in the pull request.

## 6. Hand over

Commit following the `git-commit` skill. Push the branch.

Open a draft pull request against `trunk`, titled after the ticket. When the branch already has one, update its body instead. Never mark it ready, never merge it, and never close the issue.

The body:

<pull-request-body>

Closes #<issue number>

## Acceptance criteria

- [x] <criterion>: `Test_<Subject>_should_<outcome>`
- [ ] <criterion to test by hand>: link to the issue comment
- [ ] <criterion not met>: why

## Last test run

Whether `make test-run` passed. When it failed, the failing tests and their output.

## Review findings not applied

Each finding from step 5 that was not applied, with its axis: Standards or Spec.

## Stopped on

Anything the builder stopped on, and a pointer to the question when the run short-circuited. Omit the section when there is nothing.

</pull-request-body>

When the suite still fails after the last round, open the draft pull request anyway, with the failure in "Last test run".
