# Coding does not repeat a create

Coding writes the agent and a starting event before it calls Cursor. A later tick does not call Cursor again when that agent has no ack, or when the create was rejected. The issue stays taken. This supersedes the repeat in `0008-pure-tick-steps.md` for coding. Sending a `bc-` UUID v7 as Cursor's agent id is superseded by `0042-what-cursor-allows-an-agent.md`: a token and a factory-chosen id cannot be sent together.

## Considered Options

- Repeating the same create until it is acknowledged: rejected, because a lost response should stay visible instead of being retried by the next tick.
