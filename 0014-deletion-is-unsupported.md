# Deletion is unsupported

A row in any Config table cannot be removed. If State has a builder, reviewer, fixer, App, label, or check that Config no longer has, the step throws and deletes nothing. Creating a missing row and updating one that both sides have are supported. Runs, agents, ticks, and events are not Config rows. How a removed config row should be cleaned up is deferred.

## Considered Options

- Delete the State row, and the GitHub resource when the tick created it: rejected, because deletion is unsupported.
- Keep a State row whose Config row is gone and start no new work from it: rejected, because the Config row has to be there.
