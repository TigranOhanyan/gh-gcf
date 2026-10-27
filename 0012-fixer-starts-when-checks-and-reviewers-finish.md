# The fixer starts when the listed checks and the reviewers have finished

The tick does not claim a fixer on the first failing check or the first changes-requested verdict. It waits until every listed check has finished and every reviewer is done. Then, if any listed check failed or any reviewer requested changes, it claims and starts one fixer. A check that is not listed does not count. Reviewers start when the builder's agent completes, without waiting for the checks. The builder's run stores the pull request number and head SHA. Each reviewer run stores its verdict. The tick reads listed check results from GitHub and does not store them. This supersedes `0007-fixer-claimed-before-it-starts.md`.

## Considered Options

- Claim as soon as one check fails or one reviewer requests changes, and start later: rejected, because waiting until all of them have finished still creates one fixer and is the simpler rule.
