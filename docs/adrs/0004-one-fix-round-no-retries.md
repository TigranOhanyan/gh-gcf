# One fix round, no retries

Superseded by `0036-checks-then-verdicts-then-one-fixer.md`. One fixer still runs at most once, and an error still stops for a person.

After both verdicts and CI are in, a fresh fixer runs once if any reviewer requested changes or CI is red, and the factory then stops for that pull request. Every error, including failed launches, marks the issue for attention instead of retrying. The factory starts with as few assumptions as possible and grows from what we observe.