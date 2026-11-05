package commands

import (
	"context"
	"crypto/rsa"
	"fmt"
	"os"
	"strconv"
	"time"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"
)

const factoryAppIDEnv = "FACTORY_GITHUB_APP_ID"
const factoryAppKeyEnv = "FACTORY_GITHUB_APP_PRIVATE_KEY"

// FactoryApp is the GitHub App the tick uses for its own calls.
type FactoryApp struct {
	id  int64
	key *rsa.PrivateKey
}

// Configured reports whether the factory app id and private key are both set.
func (app FactoryApp) Configured() (ok bool) {
	ok = app.key != nil
	return
}

// LoadFactoryApp reads FACTORY_GITHUB_APP_ID and FACTORY_GITHUB_APP_PRIVATE_KEY.
// Both empty means the caller uses the user token. One of them set is an error.
func LoadFactoryApp() (app FactoryApp, err error) {
	idText := os.Getenv(factoryAppIDEnv)
	pemText := os.Getenv(factoryAppKeyEnv)
	idMissing := idText == ""
	keyMissing := pemText == ""
	bothMissing := idMissing && keyMissing
	if bothMissing {
		return
	}
	incomplete := idMissing || keyMissing
	if incomplete {
		err = fmt.Errorf("github app credentials are incomplete")
		log.Error("github app", "err", err)
		return
	}

	id, err := strconv.ParseInt(idText, 10, 64)
	if err != nil {
		err = fmt.Errorf("github app id is not an integer")
		log.Error("github app", "err", err)
		return
	}
	key, err := ParseAppKey(pemText)
	if err != nil {
		log.Error("github app", "err", err)
		return
	}
	app.id = id
	app.key = key
	return
}

// Client returns an installation client for the factory app on owner/repo.
func (app FactoryApp) Client(ctx context.Context, owner string, repo string) (client *github.Client, err error) {
	now := time.Now()
	signed, err := SignAppJWT(app.id, app.key, now)
	if err != nil {
		log.Error("app jwt", "owner", owner, "name", repo, "err", err)
		return
	}
	appClient, err := NewGitHubClientWithToken(signed)
	if err != nil {
		return
	}

	installation, resp, err := appClient.Apps.GetRepositoryInstallation(ctx, owner, repo)
	CloseGitHub(resp)
	if err != nil {
		log.Error("repository installation", "owner", owner, "name", repo, "err", err)
		return
	}
	if installation == nil {
		err = fmt.Errorf("repository installation is missing")
		log.Error("repository installation", "owner", owner, "name", repo, "err", err)
		return
	}
	missingID := installation.ID == nil
	if missingID {
		err = fmt.Errorf("repository installation is missing")
		log.Error("repository installation", "owner", owner, "name", repo, "err", err)
		return
	}

	installationID := installation.GetID()
	issued, tokenResp, err := appClient.Apps.CreateInstallationToken(ctx, installationID, nil)
	CloseGitHub(tokenResp)
	if err != nil {
		log.Error("installation token", "owner", owner, "name", repo, "err", err)
		return
	}
	if issued == nil {
		err = fmt.Errorf("installation token is empty")
		log.Error("installation token", "owner", owner, "name", repo, "err", err)
		return
	}
	missingToken := issued.Token == nil || *issued.Token == ""
	if missingToken {
		err = fmt.Errorf("installation token is empty")
		log.Error("installation token", "owner", owner, "name", repo, "err", err)
		return
	}

	token := issued.GetToken()
	client, err = NewGitHubClientWithToken(token)
	return
}

// GitHubClient uses the factory app when it is configured, and fallback otherwise.
func GitHubClient(ctx context.Context, app FactoryApp, owner string, repo string, fallback *github.Client) (client *github.Client, err error) {
	useApp := app.Configured()
	if useApp {
		client, err = app.Client(ctx, owner, repo)
		return
	}
	client = fallback
	if client == nil {
		err = fmt.Errorf("github token is missing")
		log.Error("github", "owner", owner, "name", repo, "err", err)
	}
	return
}
