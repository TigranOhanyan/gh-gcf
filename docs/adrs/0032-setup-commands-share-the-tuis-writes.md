# Setup commands share the TUI's writes and reads

`gh gcf setup tui` is the TUI (`0037-the-interactive-setup-command-is-setup-tui.md`). Plain subcommands under `setup` add and list the same config rows: repos, labels, Apps, members, and checks. Each subcommand calls the same validation, write, and query the TUI calls. The TUI does not shell out to them. A subcommand writes config in the open checkout and does not commit, as in `0023-setup-records-config-in-the-checkout-that-is-open.md`. The names sit under `setup` because `gh gcf label` is already the tick step. The onboarding skill that drives these commands comes later.

The TUI fills a draft one field at a time and calls the shared write when the draft is complete. A subcommand takes every argument in one invocation and calls that same write. The draft is the partial input. It is not a builder member.

## Considered Options

- The TUI shells out to the subcommands: rejected, because the TUI would parse its own output.
- Top-level commands named like the tick steps: rejected, because `label` and `app` already run a step.
- A subcommand that prompts one field at a time: rejected, because the skill passes every value in one invocation.
