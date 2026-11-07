# Checks finish, then verdicts, then at most one fixer

The standards reviewer starts when the builder's agent has finished. The specification reviewer starts when the standards reviewer has finished. Neither waits for checks, and the tick does not read verdicts between them.

When both have finished, the tick reads check runs for the builder's head SHA in one list call. For each name listed on the development, the run with the newest `started_at` counts. A name with no run is not finished. Queued or in progress is not finished, and the tick waits. Success, skipped, and neutral have passed. Failure, cancelled, and timed out are red. A run whose name is not listed does not count. No listed names means the checks have already passed.

The tick reads verdicts only once every listed check has finished, including a red one. A verdict is that App's latest review in `APPROVED` or `CHANGES_REQUESTED`. A `COMMENTED` review is missing. A missing verdict sets the development work to `attention` and does not start the fixer. With both verdicts present, a `changes_requested` verdict or a red check starts the fixer. Two approvals and every check passed set the work to `finished` and leave the pull request as it is. The fixer link stays empty.

When the fixer has finished, the tick reads the listed checks on the fixer's head SHA and does not read reviews. A check still running waits. Every check passed sets `finished` and leaves the pull request as it is. A red check sets `attention`. The fixer does not run again.

Any error sets `attention`, applies the development's attention label to the issue, and is not retried. A check that has not finished is not an error. An instruction hash that the next compilation will replace is not an error. Merging the pull request is a later decision. A stale agent that stays started is a later decision.

This supersedes `0004-one-fix-round-no-retries.md`, `0012-fixer-starts-when-checks-and-reviewers-finish.md`, and the phase transitions in `0033-a-coding-work-records-its-status.md`.

## Considered Options

- Starting the fixer on a red check before the verdicts are read: rejected, because a missing verdict must stop for a person, including when a check is red.
- Reading verdicts again after the fixer: rejected, because the new commit drops them, and a second fixer is not run.
- Storing check runs: rejected, because the list call returns every attempt, and a copy would be stale after a rerun.
- An agent choosing the next role: rejected, because the same development work must take the same next step.
