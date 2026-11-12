package builder

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

func openFromBranch(ctx context.Context, client *github.Client, owner string, repo string, branches []cursorBranch) (number int, found bool, err error) {
	branch, ok := soleBranch(branches, owner, repo)
	if !ok {
		return
	}
	number, found, err = oneOpenPull(ctx, client, owner, repo, branch)
	return
}

func soleBranch(branches []cursorBranch, owner string, name string) (branch string, found bool) {
	var names []string
	for _, item := range branches {
		matches := sameRepo(item.RepoURL, owner, name)
		named := item.Branch != ""
		usable := matches && named
		if usable {
			names = append(names, item.Branch)
		}
	}
	one := len(names) == 1
	if !one {
		return
	}
	branch = names[0]
	found = true
	return
}

func sameRepo(raw string, owner string, name string) (matched bool) {
	needle := "/" + owner + "/" + name
	index := strings.Index(raw, needle)
	missing := index < 0
	if missing {
		return
	}
	after := index + len(needle)
	ended := after == len(raw)
	if ended {
		matched = true
		return
	}
	next := raw[after]
	matched = next == '/' || next == '?' || next == '#'
	return
}

func oneOpenPull(ctx context.Context, client *github.Client, owner string, repo string, branch string) (number int, found bool, err error) {
	opts := &github.PullRequestListOptions{
		State: "open",
		Head:  owner + ":" + branch,
	}
	pulls, resp, err := client.PullRequests.List(ctx, owner, repo, opts)
	commands.CloseGitHub(resp)
	if err != nil {
		log.Error("pull requests", "owner", owner, "name", repo, "branch", branch, "err", err)
		return
	}
	single := len(pulls) == 1
	if !single {
		return
	}
	number = pulls[0].GetNumber()
	positive := number > 0
	if !positive {
		return
	}
	found = true
	return
}

func headSHA(ctx context.Context, client *github.Client, owner string, repo string, number int) (sha string, err error) {
	pull, resp, err := client.PullRequests.Get(ctx, owner, repo, number)
	commands.CloseGitHub(resp)
	if err != nil {
		log.Error("pull request", "owner", owner, "name", repo, "pull", number, "err", err)
		return
	}
	if pull == nil {
		err = fmt.Errorf("pull request is missing")
		log.Error("pull request", "owner", owner, "name", repo, "pull", number, "err", err)
		return
	}
	sha = pull.GetHead().GetSHA()
	missing := sha == ""
	if missing {
		err = fmt.Errorf("pull request head is missing")
		log.Error("pull request", "owner", owner, "name", repo, "pull", number, "err", err)
	}
	return
}

func pullNumber(branches []cursorBranch, owner string, name string) (number int, found bool) {
	needle := "/" + owner + "/" + name + "/"
	for _, branch := range branches {
		hasRepo := strings.Contains(branch.PRURL, needle)
		if !hasRepo {
			continue
		}
		number, found = parsePullNumber(branch.PRURL)
		if found {
			return
		}
	}
	return
}

func parsePullNumber(raw string) (number int, found bool) {
	marker := "/pull/"
	index := strings.LastIndex(raw, marker)
	missing := index < 0
	if missing {
		return
	}
	tail := raw[index+len(marker):]
	tail, _, _ = strings.Cut(tail, "?")
	tail = strings.Trim(tail, "/")
	parsed, err := strconv.Atoi(tail)
	if err != nil {
		return
	}
	positive := parsed > 0
	if !positive {
		return
	}
	number = parsed
	found = true
	return
}
