# gh gcf

`gh gcf` is a [GitHub CLI](https://cli.github.com/) extension that installs and updates the [software factory](https://github.com/TigranOhanyan/github-cursor-factory) in a host repository. `pull` copies factory files into the host. `build` starts Cursor cloud agents for issues that are ready to build.

`gh gcf pull` runs from the root of the host repository. It downloads one revision of the factory as a tarball, using the GitHub API and the credentials from `gh auth login`, and copies the files it owns into the host. A lock file records that set so a later pull can update factory files and leave everything else alone. The command does not commit. You, or a workflow, commit the result.

## Install

After a release of this extension is published:

```shell
gh extension install TigranOhanyan/gh-gcf
```

`gh extension upgrade gcf` installs a newer release of the extension.

To try a checkout before a release exists, build the binary and point `gh` at it:

```shell
go build -o gh-gcf .
gh extension install .
```

## pull

`gh gcf pull` must be run from the root of the host git repository, the directory that contains `.git`. It fails from a subdirectory.

```shell
gh gcf pull <ref> [--source OWNER/REPO] [--force]
```

`<ref>` is a tag, branch, or commit SHA. A short SHA is accepted when GitHub can resolve it. `latest` means the factory repository's latest GitHub release: the newest release that is not a prerelease. If that repository has no such release, the command fails and says so. `latest` is not a branch or tag name; a ref that is literally named `latest` cannot be selected with this command.

`--source` is the factory repository in `OWNER/REPO` form. It defaults to `TigranOhanyan/github-cursor-factory`. Use it to pull a fork. The source used for the pull is stored in the lock file.

`--force` overwrites files you edited after the last pull, restores factory files you deleted, overwrites unmanaged files that already occupy a path being installed, and deletes an edited file that this pull would otherwise keep because the factory stopped shipping it. It does not apply to the reserved paths below.

The command prints a short summary of added, updated, removed, and skipped files. A skipped file also produces a warning. Nothing is committed.

### Updates

The lock file is the list of files `gh gcf` owns.

- A file that still matches its locked checksum is updated when the factory copy changed.
- A file that already matches the incoming factory bytes is left as it is, and the lock records the new checksum. That includes a local edit that happens to match the file being pulled.
- A file whose contents match neither the lock nor the incoming file is left unchanged, with a warning. The lock keeps the previous checksum so the next pull still sees the edit.
- A factory file you deleted is left deleted, with a warning. The locked checksum is unchanged.
- A file the factory no longer ships is deleted when it still matches the lock. When you have edited it, it is left in place, a warning is printed, and it is dropped from the lock. Later pulls do not touch it.
- A path that is not in the lock is never modified. The exception is a factory file that is new to this install: it is created when the path is empty, and it is a conflict when something unmanaged is already there.
- The first pull into a repository that already has a file at an install path fails, and writes nothing, unless `--force` is given. The same is true later, when the factory adds a file at a path that already exists and is not in the lock.
- `--force` is what overwrites those conflicts, local edits, and local deletions. A pull that hits a conflict or a reserved path writes nothing.

## File mapping

| Factory path | Host path |
| --- | --- |
| Files under `github/workflows/`, at any depth | The same path under `.github/workflows/` |
| Every other file | The same path under `.factory/` |
| The factory's `.github/` directory | Not copied |
| `README.md` at the factory root | Not copied |
| `.gitignore` at the factory root | Not copied |

A `README.md` or `.gitignore` in a subdirectory is copied. A file whose path is exactly `github/workflows`, rather than a file inside that directory, is copied to `.factory/github/workflows`.

With the factory's current layout, `labels.tf` is installed at `.factory/labels.tf` and `github/workflows/factory.yml` is installed at `.github/workflows/factory.yml`. The factory's own `.github/workflows/validate.yml` is not installed.

## Reserved paths

`gh gcf` never writes, deletes, or overwrites these paths, including when `--force` is set:

- `.factory/state` and anything under `.factory/state/`
- a file directly in `.factory/` whose name ends with `_override.tf`
- `.factory/gcf.lock`

Host state and override files stay where they are. If a factory archive contains a file that would be installed onto one of these paths, the pull fails and does not change the repository. A file such as `modules/labels_override.tf` is not reserved; it is installed at `.factory/modules/labels_override.tf`.

## Lock file

Each successful pull writes `.factory/gcf.lock`:

```json
{
  "version": 1,
  "source": "TigranOhanyan/github-cursor-factory",
  "ref": "v1.2.3",
  "commit": "0123456789abcdef0123456789abcdef01234567",
  "files": [
    {
      "path": ".factory/labels.tf",
      "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    },
    {
      "path": ".github/workflows/factory.yml",
      "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    }
  ]
}
```

`source` is the repository the pull used. `ref` is the argument you passed, including the literal `latest`. `commit` is the full 40-character SHA that ref resolved to. `files` lists every file this pull still manages, sorted by path. `sha256` is the lowercase hex SHA-256 of the bytes last installed for that file. It is not a checksum of later local edits. The lock file itself is not an entry in `files`.

`version` is `1`. `gh gcf` stops if the lock version is anything else.

The example digests above are the SHA-256 of an empty file, not the checksums of those factory files.

## build

`gh gcf build` is one pass over the host repository. It finds open issues that are ready to build and starts a Cursor cloud agent for each. The agent implements the issue and opens a pull request. This command does not merge that pull request, and it does not poll or schedule.

```shell
gh gcf build [--repo OWNER/REPO] [--max N] [--dry-run]
```

`--repo` is the host repository in `OWNER/REPO` form. It defaults to the current repository, the same way other `gh` commands do. `--max` is the maximum number of agents to start in this run. The default is 1. Issues are taken oldest first. `--dry-run` prints each issue that would be built and the request that would be sent. It still reads the repository. It does not add labels, post comments, or call Cursor, and it does not need `CURSOR_API_KEY`.

An issue is ready when it is open, is not a pull request, has the `todo` label, and has neither `in-progress` nor `needs-attention`. Those labels come from the factory's `labels.tf`. This command does not create them. It leaves `todo` in place. It adds `in-progress` to claim an issue, and removes that label only when Cursor returns HTTP 429.

For each selected issue, the command adds `in-progress`, then sends `POST https://api.cursor.com/v1/agents`. The request includes the issue number, title, and body, because the agent's own GitHub token cannot read issues. It also tells the agent to implement only what the issue asks, keep the change small, follow the repository's conventions, run the repository's tests or checks if any exist, never edit `.factory/` or `.github/workflows/`, and write a pull request body that says what changed, how it was checked, and includes `Closes #<n>`. The agent starts from the repository's default branch. On success, the command comments on the issue with the agent's URL.

The pull request is opened under the GitHub account of the Cursor API key's owner.

If Cursor rejects the request, the command adds `needs-attention`, comments on the issue with the HTTP status and the API error message, and continues with the next issue. Timeouts and network errors take that same path, because an agent may already exist. The command exits non-zero if any issue failed. An HTTP 429 from Cursor means no agent was created: the command removes `in-progress`, does not add `needs-attention`, does not comment, and stops the run. An HTTP 429 from GitHub also stops the run.

### Credentials

`CURSOR_API_KEY` is required, except with `--dry-run`. It is a user API key from [Cursor Dashboard → API Keys](https://cursor.com/dashboard/api). The command trims surrounding whitespace and newlines. It fails if the variable is missing, and it does not print the key.

`GCF_BUILDER_MODEL` is optional. When it is set, the command sends that value as `model.id`. When it is unset, the request omits `model` and Cursor uses the account's default model.

GitHub requests use the credentials `gh` already has, from `GH_TOKEN` or `gh auth login`.

## Releases

Pushing a tag that matches `v*` runs [`.github/workflows/release.yml`](.github/workflows/release.yml). The workflow checks out the tag and runs [`cli/gh-extension-precompile`](https://github.com/cli/gh-extension-precompile), which builds `gh-gcf` binaries for each of its platforms and uploads them to a GitHub release for that tag. `gh extension install` and `gh extension upgrade` download the binary that matches the current platform from those releases.

Publishing a release is done by pushing a version tag. This repository does not ship a release until then.
