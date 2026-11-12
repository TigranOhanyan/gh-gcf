# The builder pull request is Cursor's, then the branch's

A finished builder run reports a branch and, only sometimes, a pull request URL. The check used to require the URL, so a pull request Cursor had not filled in, or one opened with `gh` on some other branch, became attention and the reviewers never started. The runs below are why the builder create lets Cursor open the pull request, the trailer forbids `gh` for that open, and the check falls back to the one open pull request on the single reported branch.

## What we tried

On `test-gcf`, with `autoCreatePR` false, the agent called Cursor's pull request tool after pushing `cursor/issue-7-task-a5ee`. The tool registered an approval and returned no URL. GitHub had no pull request from `pr-check-default`. `gh`, with the builder App's token, opened pull request 9 from `pr-check-gh` as `app/gcf-builder`. The finished run listed only `cursor/issue-7-task-a5ee`, with no pull request URL. Pull request 8, also `app/gcf-builder`, already sat on `cursor/issue-7-task-a5ee` from an earlier turn, and the run did not report it.

A new agent, with `autoCreatePR` true and `skipReviewerRequest` true, was told to add a line to the readme and not to open a pull request. It pushed `cursor/readme-line-addition-67f9`. The finished run had no pull request URL. GitHub had the branch and no pull request.

A follow-up on that agent said to open a pull request with Cursor's tool and not with `gh`. Pull request 10 opened during that run, from `cursor/readme-line-addition-67f9` into `main`. Its author was the Cursor account, it was not a draft, and it had no requested reviewers. The finished run's branch entry included the pull request URL.

The commits were authored as the account that already appeared in the repo, after the agent replaced `Cursor Agent <cursoragent@cursor.com>`. Commit author is outside this decision. Who GitHub shows as the pull request author is too. The builder App stays the identity for the GitHub operations Cursor's tool does not do.

## What we rejected

Discovering a `gh` pull request from the branch in the response. Pull request 9 was on `pr-check-gh`. The response named a different branch.

Discovering it by the App that opened it. Pull request 10, the one that came back in the response, was the Cursor account, not a bot. A bot filter would have dropped it.

The factory opening the pull request from the reported branch after the run. The factory does not have the body the agent writes, and the reported branch is not a pull request until one exists.

Telling the fixer to open a pull request, and looking one up when the fixer finishes. The fixer already has the builder's pull request. One the fixer opens is not stored.

Storing the pull request URL beside the number. The work stores the number. The URL is the owner, the repo, and that number. The issue stores both because the number is the unique identity and the URL is handed to the agent.

Naming draft or ready in the trailer. That choice belongs in the owner's instruction in the factory repo. The extension does not police an instruction that overrides the trailer. A draft is open, so the branch lookup accepts it.

## What we decided

The builder create sends `autoCreatePR` true and `skipReviewerRequest` true. The fixer create and both reviewer creates keep `autoCreatePR` false and omit `skipReviewerRequest`. The builder trailer says not to open the pull request with `gh`. It is not written into the instruction file. The fixer trailer says nothing about which tool opens a pull request.

The builder check takes a pull request URL from the run when the repo matches. When none matches and the run has exactly one branch for this app repo, it lists open pull requests with head `owner:branch` and stores the number only when that list has one. No URL and no single branch, or a list that is not exactly one open pull request, sets attention and stores no number. A GitHub error fails the check and leaves the work untouched. The fixer check does not look again.

`0042-what-cursor-allows-an-agent.md` says the factory does not use the Cursor branch. That stands for every other use. This check uses the branch as the fallback head.
