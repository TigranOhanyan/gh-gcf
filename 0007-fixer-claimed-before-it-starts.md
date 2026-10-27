# The fixer is claimed before it is started

Superseded by `0012-fixer-starts-when-checks-and-reviewers-finish.md`. The tick waits until the listed checks and the reviewers have finished, then claims and starts one fixer.

A tick claims one fixer for a builder's run as soon as any reviewer requests changes or CI fails. The claim is an idempotent upsert, so two reviewers requesting changes still create one fixer. The tick does not start that fixer until every reviewer is done and CI has finished.

## Considered Options

- Claim and start in one step after every reviewer is done: rejected, because the claim and the start are different steps, and a second reviewer requesting changes must not create a second fixer.
