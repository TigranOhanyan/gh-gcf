# A lost reviewer create stays lost

A reviewer create whose response never arrives is left as it is. The next tick does not create again and does not set attention. The issue stays taken. This is the same outcome as `0031-coding-does-not-repeat-a-create.md`. Setting attention an hour after the agent row was created is deferred.

## Considered Options

- Setting attention on the first tick an hour after the agent was created: deferred, because a lost response has no Cursor id to recover, and the hour can be added later without changing the create.
