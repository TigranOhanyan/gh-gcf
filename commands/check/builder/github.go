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
