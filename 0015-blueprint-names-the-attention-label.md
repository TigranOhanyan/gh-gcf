# A blueprint names the attention label

Each builder, reviewer, and fixer names the label to apply when its run cannot proceed. The tick applies that label to the issue and halts that run. The blueprint row does not change, so another issue for the same blueprint can still start.

## Considered Options

- A status column on the blueprint: rejected, because one failed run would stop every issue for that blueprint, and the tick would be writing Config.
