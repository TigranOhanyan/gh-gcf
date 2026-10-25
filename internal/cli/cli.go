// Package cli is the gh gcf command line.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/TigranOhanyan/gh-gcf/internal/pull"
	"github.com/TigranOhanyan/gh-gcf/internal/source"
)

const usageText = `gh gcf installs and updates software factory files in a host repository.

Usage:
  gh gcf pull <ref> [--source OWNER/REPO] [--force]

Commands:
  pull <ref>    Download the factory at <ref> and install it into this repository

<ref> is a tag, branch, or commit SHA. "latest" means the newest GitHub release.
The command must be run from the root of the host git repository.
It does not commit.

Flags:
  --source OWNER/REPO    Factory repository (default TigranOhanyan/github-cursor-factory)
  --force                Overwrite local edits and unmanaged files that are in the way
`

// Fetcher downloads a resolved factory archive.
type Fetcher interface {
	Fetch(ctx context.Context, repo, ref string) (source.Fetched, error)
}

// Run executes gh gcf with args. fetch may be nil to use the GitHub API.
func Run(args []string, cwd string, stdout, stderr io.Writer, fetch Fetcher) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return errors.New("a command is required")
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usageText)
		return nil
	case "pull":
		return runPull(args[1:], cwd, stdout, stderr, fetch)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runPull(args []string, cwd string, stdout, stderr io.Writer, fetch Fetcher) error {
	ref, repo, force, help, err := parsePullArgs(args)
	if err != nil {
		return err
	}
	if help {
		fmt.Fprint(stdout, usageText)
		return nil
	}
	if err := repoRoot(cwd); err != nil {
		return err
	}
	if fetch == nil {
		fetch = source.GitHub{}
	}
	fetched, err := fetch.Fetch(context.Background(), repo, ref)
	if err != nil {
		return err
	}
	summary, err := pull.Pull(pull.Request{
		Dir:     cwd,
		Source:  repo,
		Ref:     ref,
		Commit:  fetched.Commit,
		Force:   force,
		Archive: fetched.Archive,
	})
	if err != nil {
		return err
	}
	for _, warning := range summary.Warnings {
		fmt.Fprintf(stderr, "warning: %s\n", warning)
	}
	fmt.Fprint(stdout, FormatSummary(repo, ref, fetched.Commit, fetched.ReleaseTag, summary))
	return nil
}

func parsePullArgs(args []string) (ref, repo string, force, help bool, err error) {
	repo = source.DefaultRepo
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help" || arg == "-help":
			return "", "", false, true, nil
		case arg == "--force" || arg == "-force":
			force = true
		case arg == "--force=true" || arg == "-force=true":
			force = true
		case arg == "--force=false" || arg == "-force=false":
			force = false
		case arg == "--source" || arg == "-source":
			i++
			if i >= len(args) || strings.HasPrefix(args[i], "-") {
				return "", "", false, false, errors.New("flag needs an argument: --source")
			}
			repo = args[i]
		case strings.HasPrefix(arg, "--source="):
			repo = strings.TrimPrefix(arg, "--source=")
		case strings.HasPrefix(arg, "-source="):
			repo = strings.TrimPrefix(arg, "-source=")
		case strings.HasPrefix(arg, "-") && arg != "-":
			return "", "", false, false, fmt.Errorf("unknown flag: %s", arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	if len(positionals) != 1 {
		return "", "", false, false, errors.New("usage: gh gcf pull <ref> [--source OWNER/REPO] [--force]")
	}
	return positionals[0], repo, force, false, nil
}

func repoRoot(dir string) error {
	if _, err := os.Lstat(filepath.Join(dir, ".git")); err != nil {
		if os.IsNotExist(err) {
			return errors.New("pull must be run from the root of a host git repository")
		}
		return err
	}
	return nil
}

// FormatSummary renders the pull result. release is printed only when ref was "latest".
func FormatSummary(repo, ref, commit, release string, summary pull.Summary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "source: %s\n", repo)
	fmt.Fprintf(&b, "ref: %s\n", ref)
	if release != "" {
		fmt.Fprintf(&b, "release: %s\n", release)
	}
	fmt.Fprintf(&b, "commit: %s\n", commit)
	b.WriteByte('\n')
	writeList(&b, "added", summary.Added)
	writeList(&b, "updated", summary.Updated)
	writeList(&b, "removed", summary.Removed)
	writeList(&b, "skipped", summary.Skipped)
	fmt.Fprintf(&b, "%d added, %d updated, %d removed, %d skipped\n",
		len(summary.Added), len(summary.Updated), len(summary.Removed), len(summary.Skipped))
	return b.String()
}

func writeList(b *strings.Builder, label string, paths []string) {
	if len(paths) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", label)
	for _, path := range paths {
		fmt.Fprintf(b, "  %s\n", path)
	}
}
