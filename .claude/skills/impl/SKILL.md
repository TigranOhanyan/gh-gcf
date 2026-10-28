---
name: impl
description: >-
  Build one agreed ticket. A builder subagent works its acceptance
  criteria test-first, then review and commit.
disable-model-invocation: true
---

# Implement

Build the ticket the user names. Invoking this skill is the agreement to build it: the ticket's acceptance criteria and the spec's contracts are settled.

You are the orchestrator. The builder subagent writes the tests and the code. You check its work, run the review, and commit. Run this skill in the main chat, so the review can start its own subagents.

## 1. Check the inputs

Read the ticket and the spec. Read `docs/agents/domain.md` and follow it: `GLOSSARY.md`, and the ADRs in `docs/adr/` that touch the area. Read `docs/agents/issue-tracker.md` when the ticket is a GitHub issue.

- The ticket: its acceptance criteria are the slices to build.
- The spec: its contracts (signatures, tables, API) and the seams where tests go.

Stop and ask the user when the ticket has no acceptance criteria, or the spec has no contract for what a criterion needs.

## 2. Dispatch the builder

Start one builder subagent in the foreground. Give it the brief in `docs/agents/builder-brief.md`, with the paths filled in. Pass pointers, not copies: the subagent reads the ticket and the spec itself.

## 3. Check the work

Read the diff yourself. Every criterion has a test named after it. Run the package's tests. Then run the full suite with `make test-run`.

When a check fails, dispatch a fresh builder with the brief, the failure, and the report.

## 4. Review

Mark the files the builder created with `git add -N <path>`, so the review sees them. Then follow `.claude/skills/code-review/SKILL.md` against the branch point, with the ticket as the spec. The review runs in its own subagents, never in this chat. When that file is missing, stop and tell the user.

Show the user the findings. Dispatch a fresh builder with the brief and the findings the user picks. Check the work again, as in step 3.

## 5. Commit

Show the user the criteria, the test names, and the files changed. Commit after the user agrees, following the `git-commit` skill.
