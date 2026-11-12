package agent

import (
	"context"

	"go.uber.org/zap"

	"github.com/TigranOhanyan/gh-gcf/logging"
	"github.com/google/go-github/v93/github"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

const pageSize = 100

// ListOpenIssues returns open issues that carry label, including later pages.
func ListOpenIssues(ctx context.Context, client *github.Client, owner string, repo string, label string) (issues []*github.Issue, err error) {
	page := 1
	for {
		options := github.IssueListByRepoOptions{
			State:  "open",
			Labels: []string{label},
			ListOptions: github.ListOptions{
				Page:    page,
				PerPage: pageSize,
			},
		}
		batch, resp, callErr := client.Issues.ListByRepo(ctx, owner, repo, &options)
		commands.CloseGitHub(resp)
		if callErr != nil {
			err = callErr
			logging.From(ctx).Error("list issues", zap.Any("owner", owner), zap.Any("name", repo), zap.Any("label", label), zap.Error(err))
			return
		}
		issues = append(issues, batch...)
		short := len(batch) < pageSize
		if short {
			return
		}
		page = page + 1
	}
}
