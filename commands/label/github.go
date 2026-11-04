package label

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
