---
name: coding
description: >-
  Command packages for this repo. Main commands are bind, setup, run, compile, and check.
  A command follows commands/run/builder/builder.go. A subcommand is its own package,
  at most three packages deep; deeper subcommands are files. Each main command
  builds its own bundle. Root adds bind, check, compile, run, and setup.
  Command tests run plain shell lines through
  the root command with Shell and Git. Breaking changes outside the current
  subissue of issue 21 are commented out. Schema changes go into the existing
  migration file. A new table gets a new file. Use when adding or moving a
  command, package, helper, or command test, when editing a migration, or when
  a change would break current behavior.
---

# Coding

Existing code does not match this convention. Leave it. Do not reshape a package, a helper, or a test to match unless that code is part of the area you are changing.

## Commands

The main commands are packages under `commands`:

```text
commands/bind
commands/setup
commands/run
commands/compile
commands/check
```

Write a command like [commands/run/builder/builder.go](../../../commands/run/builder/builder.go). It reads the working directory, requires a checkout, and hands one step to the executor.

A subcommand of a main command is its own package. Three packages is the deepest the tree goes. `commands/setup/development` and `commands/compile/development` are that depth. A further subcommand is a file in that package, not another package. `setup development add` is `add.go`. `setup development check add` is `add_check.go`. `compile development` is the package itself.

## Helpers

Each of bind, check, compile, run, and setup builds its own command and adds its subcommands. The root command adds those five and does not add their subcommands.

A helper used by a command package and its subcommands stays out of the command package, because that package imports the subcommands. Setup helpers that every action uses are package `common` in `commands/setup/common`. A write or a list for one action lives in that action's package, and the TUI imports the action package and calls the same function. Run helpers are package `agent` in `commands/run/agent`. Do not add another shared package.

A subcommand imports the helper package. It does not import the command package that imports the subcommand.

## Tests

A command test runs the root command. Wrap `root.NewRoot()` with `commands.NewCmd` and pass one shell line to `Shell`. Git steps go through `commands.Git` with one shell line. The line reads like a person typed it. A quoted value stays one argument.

Build a line with `fmt.Sprintf`. Do not join it with `+`. A line built across a loop may use `strings.Builder`.

```go
line := fmt.Sprintf("setup development add --owner octo --repo app --name %s --starting %s --attention %s --standards-app %s --specification-app %s --builder-app %s --fixer-app %s", name, starting, attention, standardsApp, specificationApp, builderApp, fixerApp)
_, err := runSetup(t, line)
```

The test file stays next to the command, in an external test package, so it can import the root command.

## Breaking changes

[Issue 21](https://github.com/TigranOhanyan/gh-gcf/issues/21) is the whole refactor, landed one subissue at a time. A breaking change that the current subissue does not need is commented out. The later subissue that owns it uncomments the change and fixes it.

## Migrations

A change to a table that already has a migration is edited into that file. Do not add a new migration file for it. A new table or concept that has no migration file yet gets its own file.
