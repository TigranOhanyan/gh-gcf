package label

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"
)

const githubAPIURL = "GITHUB_API_URL"

func newGitHubClient(ctx context.Context) (client *github.Client, err error) {
	token, err := githubToken(ctx)
	if err != nil {
		return
	}

	auth := github.WithAuthToken(token)
	options := []github.ClientOptionsFunc{auth}
	base := os.Getenv(githubAPIURL)
	hasBase := base != ""
	if hasBase {
		baseURL := withTrailingSlash(base)
		urls := github.WithURLs(&baseURL, nil)
		options = append(options, urls)
	}

	client, err = github.NewClient(options...)
	if err != nil {
		log.Error("github client", "err", err)
	}
	return
}

func withTrailingSlash(base string) (url string) {
	url = base
	missing := !strings.HasSuffix(url, "/")
	if missing {
		url = url + "/"
	}
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

func labelMissing(err error) (missing bool) {
	response, ok := err.(*github.ErrorResponse)
	if !ok {
		return
	}
	if response.Response == nil {
		return
	}
	missing = response.Response.StatusCode == http.StatusNotFound
	return
}

func closeGitHub(resp *github.Response) {
	if resp == nil {
		return
	}
	if resp.Response == nil {
		return
	}
	err := resp.Body.Close()
	if err != nil {
		log.Error("close github response", "err", err)
	}
}
