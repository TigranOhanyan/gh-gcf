package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

const pageSize = 100

func newGitHubClient(ctx context.Context) (client *github.Client, err error) {
	token, err := githubToken(ctx)
	if err != nil {
		return
	}
	client, err = commands.NewGitHubClientWithToken(token)
	return
}

func githubToken(ctx context.Context) (token string, err error) {
	token = os.Getenv("GH_TOKEN")
	if token != "" {
		return
	}
	token = os.Getenv("GITHUB_TOKEN")
	if token != "" {
		return
	}
	token, err = ghAuthToken(ctx)
	return
}

func ghAuthToken(ctx context.Context) (token string, err error) {
	exe := os.Getenv("GH_PATH")
	if exe == "" {
		exe = "gh"
	}
	cmd := exec.CommandContext(ctx, exe, "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		log.Error("gh auth token", "err", err)
		err = fmt.Errorf("github token is missing")
		return
	}
	token = strings.TrimSpace(string(output))
	missing := token == ""
	if missing {
		err = fmt.Errorf("github token is missing")
		log.Error("gh auth token", "err", err)
	}
	return
}

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
