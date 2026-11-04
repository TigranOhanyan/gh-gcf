package label

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

const appIDEnv = "FACTORY_GITHUB_APP_ID"
const appKeyEnv = "FACTORY_GITHUB_APP_PRIVATE_KEY"

type appAuth struct {
	id  int64
	key *rsa.PrivateKey
}

func loadAppAuth() (app appAuth, err error) {
	idText := os.Getenv(appIDEnv)
	pemText := os.Getenv(appKeyEnv)
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
	key, err := commands.ParseAppKey(pemText)
	if err != nil {
		log.Error("github app", "err", err)
		return
	}
	app.id = id
	app.key = key
	return
}

func (app appAuth) clientFor(ctx context.Context, owner string, repo string) (client *github.Client, err error) {
	now := time.Now()
	signed, err := commands.SignAppJWT(app.id, app.key, now)
	if err != nil {
		log.Error("app jwt", "owner", owner, "name", repo, "err", err)
		return
	}
	appClient, err := commands.NewGitHubClientWithToken(signed)
	if err != nil {
		return
	}

	installation, resp, err := appClient.Apps.GetRepositoryInstallation(ctx, owner, repo)
	commands.CloseGitHub(resp)
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
	issued, resp, err := appClient.Apps.CreateInstallationToken(ctx, installationID, nil)
	commands.CloseGitHub(resp)
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
	client, err = commands.NewGitHubClientWithToken(token)
	return
}
