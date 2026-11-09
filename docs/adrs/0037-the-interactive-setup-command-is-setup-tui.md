# The interactive setup command is setup tui

The interactive Setup is the TUI. Its command is `gh gcf setup tui`, and its package is `commands/setup/tui`. `gh gcf setup` with no subcommand prints help. The repo, label, apps, and development subcommands stay under `setup` and still call the same writes and reads as the TUI (`0032-setup-commands-share-the-tuis-writes.md`). This replaces that ADR's decision that `gh gcf setup` itself was the TUI.

## Considered Options

- `gh gcf setup` with no subcommand stays the TUI: rejected, because the interactive program is its own command and its own package.
- A top-level `gh gcf tui` beside `setup`: rejected, because the TUI is Setup done interactively, and the other recording commands stay under `setup`.
