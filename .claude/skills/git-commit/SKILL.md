---
name: git-commit
description: >-
  Commit messages: a short lowercase type and optional scope,
  one subject line, a body only when the change is large and
  non-obvious, and no footer.
  Use when the user asks for a commit or a commit message, or when
  writing a git commit.
---

# Git commit

## This repository

The subject is the message. Add a body only when the change is large and the reason is not obvious from the subject. Do not add a footer.

The scope is the part of this repo the commit changes. Omit it, and the parentheses, when the change does not sit in one part.

When infrastructure that has merged breaks in an environment, the commit is `fix(infra)`.

A change to a skill, including a new one, uses the type `skill`.

```text
test(embedder): persist embeddings and ingestion events
chore: remove unused scripts.go
fix(assistant): correct span names
fix(infra): restore the ecs service after the deploy
skill: add the git commit convention
```

## Format of the commit message

```text
<type>(<scope>): <subject>

<body>
```

## Example commit message

```text
fix(middleware): ensure Range headers adhere more closely to RFC 2616

Add one new dependency, use `range-parser` (Express dependency) to compute
range. It is more well-tested in the wild.
```

## Message subject (first line)

The first line cannot be longer than 70 characters, the second line is always blank and other lines should be wrapped at 80 characters. The type and scope should always be lowercase as shown below.

The subject:

- uses the imperative, present tense: “change” not “changed” nor “changes”
- includes motivation for the change and contrasts with previous behavior

### Allowed `<type>` values

- **feat** (new feature for the user, not a new feature for build script)
- **fix** (bug fix for the user, not a fix to a build script)
- **docs** (changes to the documentation)
- **style** (formatting, missing semi colons, etc; no production code change)
- **refactor** (refactoring production code, eg. renaming a variable)
- **test** (adding missing tests, refactoring tests; no production code change)
- **chore** (updating grunt tasks etc; no production code change)
- **skill** (adding or changing a skill)

### Example `<scope>` values

- knowledge
- embedder
- scraper
- uploader
- assistant
- chat
- embeddings
- server
- infra

The `<scope>` can be empty (e.g. if the change is a global or difficult to assign to a single component), in which case the parentheses are omitted.

## Message body

The body:

- uses the imperative, present tense: “change” not “changed” nor “changes”
- includes motivation for the change and contrasts with previous behavior

## Message footer

Do not add a footer to the commit message.

The agent never puts itself in the commit message. It is not the author and it is not a co-author. Do not add `Co-authored-by`, `Signed-off-by`, or any other trailer that names the agent.

The author is the person who takes responsibility for the code. It does not matter whether they wrote it, copied it, asked someone else to write it, or asked an agent to write it. A party that cannot take responsibility for the code, cover financial damage caused by it, or be fired or sued for it, cannot be an author or a co-author.
