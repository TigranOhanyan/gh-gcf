package fixer

import (
	"context"
	"fmt"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

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
