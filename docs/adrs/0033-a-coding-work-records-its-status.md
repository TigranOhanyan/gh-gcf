# A coding work records its status

Superseded by `0035-a-development-work-records-two-outcomes.md` and `0036-checks-then-verdicts-then-one-fixer.md`. A development work is `attention` or `finished`.

A coding work has a status the tick queries. It starts as `building`. Storing the pull request number and `builder_head_sha` moves it to `reviewing`. The last reviewer verdict moves it to `checking`. A `changes_requested` verdict or a failed listed check moves it to `fixing` and the fixer starts. `fixer_head_sha` is the commit that fixer pushed. When that commit's listed checks have all completed, or when every reviewer approved and every listed check on the builder's commit succeeded, the status becomes `finished`. A stop for a person sets `attention`. Reviewers stay a list, and a verdict stays on the reviewer agent. The tick reads listed check runs from GitHub and does not store them. One list call for a SHA is enough. The app keeps the listed names and, for each name, the run with the newest `started_at`.

This supersedes the single head SHA in `0030-members-and-agents-replace-blueprints-and-runs.md` and `0012-fixer-starts-when-checks-and-reviewers-finish.md`.

## Considered Options

- `remaining_reviews` and `remaining_checks` on the work: rejected, because a flaky check runs again and a countdown cannot return, and the review count can disagree with the verdict rows.
- A table of check runs copied from GitHub: rejected, because the list call already returns every attempt, and the copy would be stale after a rerun.
