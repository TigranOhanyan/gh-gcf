package fixer

import (
	"context"
	"crypto/rsa"
	"fmt"
	"os"
	"strconv"
	"time"

	"charm.land/log/v2"
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
		log.Error("check runs", "owner", owner, "name", repo, "sha", sha, "err", err)
		return
	}
	if result == nil {
		return
	}
	runs = result.CheckRuns
	return
}

const reviewPageSize = 100
const keyPrefix = "FACTORY_GITHUB_APP_"
const verdictApprove = "approve"
const verdictChanges = "changes_requested"
const reviewApproved = "APPROVED"
const reviewChanges = "CHANGES_REQUESTED"

func reviewerVerdict(ctx context.Context, owner string, repo string, number int, appID int64, installationID int64) (verdict string, found bool, err error) {
	client, botID, err := reviewerClient(ctx, appID, installationID)
	if err != nil {
		return
	}
	reviews, err := listReviews(ctx, client, owner, repo, number)
	if err != nil {
		return
	}
	verdict, found = standingVerdict(reviews, botID)
	return
}

func reviewerClient(ctx context.Context, appID int64, installationID int64) (client *github.Client, botID int64, err error) {
	key, err := loadAppKey(appID)
	if err != nil {
		return
	}
	now := time.Now()
	signed, err := commands.SignAppJWT(appID, key, now)
	if err != nil {
		log.Error("app jwt", "app", appID, "err", err)
		return
	}
	appClient, err := commands.NewGitHubClientWithToken(signed)
	if err != nil {
		return
	}
	issued, tokenResp, err := appClient.Apps.CreateInstallationToken(ctx, installationID, nil)
	commands.CloseGitHub(tokenResp)
	if err != nil {
		log.Error("installation token", "app", appID, "installation", installationID, "err", err)
		return
	}
	token := ""
	if issued != nil {
		token = issued.GetToken()
	}
	missingToken := token == ""
	if missingToken {
		err = fmt.Errorf("installation token is empty")
		log.Error("installation token", "app", appID, "installation", installationID, "err", err)
		return
	}
	client, err = commands.NewGitHubClientWithToken(token)
	if err != nil {
		return
	}
	user, userResp, err := client.Users.Get(ctx, "")
	commands.CloseGitHub(userResp)
	if err != nil {
		log.Error("app user", "app", appID, "err", err)
		return
	}
	botID = user.GetID()
	missingUser := botID == 0
	if missingUser {
		err = fmt.Errorf("app user is missing")
		log.Error("app user", "app", appID, "err", err)
	}
	return
}

func loadAppKey(id int64) (key *rsa.PrivateKey, err error) {
	name := keyPrefix + strconv.FormatInt(id, 10)
	pemText := os.Getenv(name)
	missing := pemText == ""
	if missing {
		err = fmt.Errorf("github app key is missing")
		log.Error("github app", "app", id, "err", err)
		return
	}
	key, err = commands.ParseAppKey(pemText)
	if err != nil {
		log.Error("github app", "app", id, "err", err)
	}
	return
}

func listReviews(ctx context.Context, client *github.Client, owner string, repo string, number int) (reviews []*github.PullRequestReview, err error) {
	page := 1
	for {
		options := github.ListOptions{
			Page:    page,
			PerPage: reviewPageSize,
		}
		var batch []*github.PullRequestReview
		var resp *github.Response
		batch, resp, err = client.PullRequests.ListReviews(ctx, owner, repo, number, &options)
		commands.CloseGitHub(resp)
		if err != nil {
			log.Error("list reviews", "owner", owner, "name", repo, "pull", number, "err", err)
			return
		}
		reviews = append(reviews, batch...)
		short := len(batch) < reviewPageSize
		if short {
			return
		}
		page = page + 1
	}
}

func standingVerdict(reviews []*github.PullRequestReview, botID int64) (verdict string, found bool) {
	var chosen *github.PullRequestReview
	for _, review := range reviews {
		present := review != nil
		authorID := int64(0)
		if present {
			authorID = review.GetUser().GetID()
		}
		sameAuthor := present && authorID == botID
		if !sameAuthor {
			continue
		}
		mapped, counted := verdictOf(review.GetState())
		if !counted {
			continue
		}
		later := laterReview(review, chosen)
		if later {
			chosen = review
			verdict = mapped
			found = true
		}
	}
	return
}

func verdictOf(state string) (verdict string, counted bool) {
	approved := state == reviewApproved
	if approved {
		verdict = verdictApprove
		counted = true
		return
	}
	changes := state == reviewChanges
	if changes {
		verdict = verdictChanges
		counted = true
	}
	return
}

func laterReview(review *github.PullRequestReview, chosen *github.PullRequestReview) (later bool) {
	if chosen == nil {
		later = true
		return
	}
	submitted := review.GetSubmittedAt().Time
	chosenAt := chosen.GetSubmittedAt().Time
	later = !submitted.Before(chosenAt)
	return
}
