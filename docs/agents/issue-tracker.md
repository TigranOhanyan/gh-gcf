# Issue tracker: GitHub

Issues and specs for this repo live as GitHub issues. Use the `gh` CLI for all operations.

## Conventions

- **Create an issue**: `gh issue create --title "..." --body "..."`. Use a heredoc for multi-line bodies.
- **Create a ticket under a spec**: `gh issue create --title "..." --body "..." --parent <spec> --blocked-by <n>,<m> --label ready-for-agent`. `--parent` makes it a native sub-issue of the spec; `--blocked-by` sets the native blocking edges. Omit `--blocked-by` when nothing blocks it.
- **Link an existing issue as a sub-issue**: `gh api -X POST repos/{owner}/{repo}/issues/<parent>/sub_issues -F sub_issue_id="$(gh api repos/{owner}/{repo}/issues/<child> --jq .id)"`. `sub_issue_id` is the issue's database id, not its number.
- **Read an issue**: `gh issue view <number> --comments`, filtering comments by `jq` and also fetching labels.
- **List issues**: `gh issue list --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'` with appropriate `--label` and `--state` filters.
- **Comment on an issue**: `gh issue comment <number> --body "..."`
- **Apply / remove labels**: `gh issue edit <number> --add-label "..."` / `--remove-label "..."`
- **Close**: `gh issue close <number> --comment "..."`

Infer the repo from `git remote -v`; `gh` does this automatically when run inside a clone.

## When a skill says "publish to the issue tracker"

Create a GitHub issue.

## When a skill says "fetch the relevant ticket"

Run `gh issue view <number> --comments`.
