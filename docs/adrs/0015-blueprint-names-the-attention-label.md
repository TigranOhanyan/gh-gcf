# A blueprint names the attention label

Superseded by `0034-a-development-is-the-flow.md`. A development has one attention label, applied to the issue whose development work stopped.

Each builder, reviewer, and fixer names the label to apply when its run cannot proceed. The tick applies that label to the issue and halts that run. The blueprint row does not change, so another issue for the same blueprint can still start.

The member names that attention label, and the tick halts that agent (`0030-members-and-agents-replace-blueprints-and-runs.md`).

## Considered Options

- A status column on the blueprint: rejected, because one failed run would stop every issue for that blueprint, and the tick would be writing Config.
