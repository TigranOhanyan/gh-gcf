# The builder does not repeat a create

The builder writes the agent and a starting event before it calls Cursor, and sends a `bc-` UUID v7 as Cursor's agent id. A later tick does not call Cursor again when that agent has no ack, or when the create was rejected. The issue stays taken. This supersedes the repeat in `0008-pure-tick-steps.md` for the builder.

## Considered Options

- Repeating the same create until it is acknowledged: rejected, because a lost response should stay visible instead of being retried by the next tick.
