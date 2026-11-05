package coding

import (
	"context"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

const pageSize = 100

func listOpenIssues(ctx context.Context, client *github.Client, owner string, repo string, label string) (issues []*github.Issue, err error) {
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
			log.Error("list issues", "owner", owner, "name", repo, "label", label, "err", err)
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

func referencedByPullRequest(ctx context.Context, client *github.Client, owner string, repo string, number int) (linked bool, err error) {
	page := 1
	for {
		options := github.ListOptions{
			Page:    page,
			PerPage: pageSize,
		}
		batch, resp, callErr := client.Issues.ListIssueTimeline(ctx, owner, repo, number, &options)
		commands.CloseGitHub(resp)
		if callErr != nil {
			err = callErr
			log.Error("issue timeline", "owner", owner, "name", repo, "issue", number, "err", err)
			return
		}
		for _, item := range batch {
			found := timelinePullRequest(item)
			if found {
				linked = true
				return
			}
		}
		short := len(batch) < pageSize
		if short {
			return
		}
		page = page + 1
	}
}

func timelinePullRequest(item *github.Timeline) (linked bool) {
	if item == nil || item.Event == nil {
		return
	}
	kind := *item.Event
	cross := kind == "cross-referenced" || kind == "connected"
	if !cross {
		return
	}
	if item.Source == nil || item.Source.Issue == nil {
		return
	}
	linked = item.Source.Issue.IsPullRequest()
	return
}

func addAttention(ctx context.Context, client *github.Client, owner string, repo string, number int, label string) (err error) {
	labels := []string{label}
	_, resp, err := client.Issues.AddLabelsToIssue(ctx, owner, repo, number, labels)
	commands.CloseGitHub(resp)
	if err != nil {
		log.Error("attention label", "owner", owner, "name", repo, "issue", number, "label", label, "err", err)
	}
	return
}
