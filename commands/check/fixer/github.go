package fixer

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/TigranOhanyan/gh-gcf/logging"
	"github.com/google/go-github/v93/github"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

const checkFilterAll = "all"
const checkPageSize = 100

func listCheckRuns(ctx context.Context, client *github.Client, owner string, repo string, sha string) (runs []*github.CheckRun, err error) {
	filter := checkFilterAll
	page := github.ListOptions{PerPage: checkPageSize}
	options := &github.ListCheckRunsOptions{
		Filter:      &filter,
		ListOptions: page,
	}
	result, resp, err := client.Checks.ListCheckRunsForRef(ctx, owner, repo, sha, options)
	commands.CloseGitHub(resp)
	if err != nil {
		logging.From(ctx).Error("check runs", zap.Any("owner", owner), zap.Any("name", repo), zap.Any("sha", sha), zap.Error(err))
		return
	}
	if result == nil {
		return
	}
	runs = result.CheckRuns
	return
}

func headSHA(ctx context.Context, client *github.Client, owner string, repo string, number int) (sha string, err error) {
	pull, resp, err := client.PullRequests.Get(ctx, owner, repo, number)
	commands.CloseGitHub(resp)
	if err != nil {
		logging.From(ctx).Error("pull request", zap.Any("owner", owner), zap.Any("name", repo), zap.Any("pull", number), zap.Error(err))
		return
	}
	if pull == nil {
		err = fmt.Errorf("pull request is missing")
		logging.From(ctx).Error("pull request", zap.Any("owner", owner), zap.Any("name", repo), zap.Any("pull", number), zap.Error(err))
		return
	}
	sha = pull.GetHead().GetSHA()
	missing := sha == ""
	if missing {
		err = fmt.Errorf("pull request head is missing")
		logging.From(ctx).Error("pull request", zap.Any("owner", owner), zap.Any("name", repo), zap.Any("pull", number), zap.Error(err))
	}
	return
}
