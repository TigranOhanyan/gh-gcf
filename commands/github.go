package commands

import (
	"net/http"
	"os"
	"strings"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"
)

const githubAPIURL = "GITHUB_API_URL"

// NewGitHubClientWithToken returns a client whose requests send token.
// GITHUB_API_URL, when set, replaces the API host.
func NewGitHubClientWithToken(token string) (client *github.Client, err error) {
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

// GitHubNotFound reports whether err is a GitHub 404.
func GitHubNotFound(err error) (missing bool) {
	if err == nil {
		return
	}
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

// CloseGitHub closes the body of a GitHub response.
func CloseGitHub(resp *github.Response) {
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

func withTrailingSlash(base string) (url string) {
	url = base
	missing := !strings.HasSuffix(url, "/")
	if missing {
		url = url + "/"
	}
	return
}
