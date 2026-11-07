---
name: layout
description: >-
  Command package layout for this repo. Main commands are bind, setup, and run.
  A subcommand is its own package, at most three packages deep; deeper
  subcommands are files. Helpers stay in the parent package and are exported.
  Command tests run plain shell lines through the root command with Shell and Git.
  Breaking changes outside the current subissue of issue 21 are commented out.
  Schema changes go into the existing migration file. A new table gets a new file.
  Use when adding or moving a command, package, helper, or command test,
  when editing a migration, or when a change would break current behavior.
---

# Layout

Existing code does not match this convention. Leave it. Do not reshape a package, a helper, or a test to match unless that code is part of the area you are changing.

## Commands

The main commands are packages under `commands`:

```text
commands/bind
commands/setup
commands/run
```

A subcommand of a main command is its own package. Three packages is the deepest the tree goes. `commands/setup/development` is that depth: `commands`, `setup`, `development`. A further subcommand is a file in that package, not another package. `setup development add` is `add.go`. `setup development check add` is `add_check.go`.

## Helpers

A helper used by a command package and its subcommand packages stays in the parent package and is exported. For setup, that is package `setup`. Do not add a `shared` package.

A child package may import its parent. The parent does not import the child. The root command attaches the child, so the two packages do not import each other.

## Tests

A command test runs the root command. Wrap `root.NewRoot()` with `commands.NewCmd` and pass one shell line to `Shell`. Git steps go through `commands.Git` with one shell line. The line reads like a person typed it. A quoted value stays one argument.

Build a line with `fmt.Sprintf`. Do not join it with `+`. A line built across a loop may use `strings.Builder`.

```go
line := fmt.Sprintf("setup development add --owner octo --repo app --name %s --starting %s --attention %s --standards-app %s --specification-app %s", name, starting, attention, standardsApp, specificationApp)
_, err := runSetup(t, line)
```

The test file stays next to the command, in an external test package, so it can import the root command.

## Breaking changes

[Issue 21](https://github.com/TigranOhanyan/gh-gcf/issues/21) is the whole refactor, landed one subissue at a time. A breaking change that the current subissue does not need is commented out. The later subissue that owns it uncomments the change and fixes it.

## Migrations

A change to a table that already has a migration is edited into that file. Do not add a new migration file for it. A new table or concept that has no migration file yet gets its own file.
