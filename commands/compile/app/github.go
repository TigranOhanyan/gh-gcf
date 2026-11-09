package app

import (
	"context"
	"crypto/rsa"
	"fmt"
	"os"
	"strconv"
	"time"

	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

const keyPrefix = "GITHUB_APP_"

func (step appStep) lookUp(ctx context.Context, owner string, repoName string, id int64) (seen sight, clientID string, installationID int64, err error) {
	key, err := loadAppKey(id)
	if err != nil {
		return
	}

	now := time.Now()
	signed, err := commands.SignAppJWT(id, key, now)
	if err != nil {
		log.Error("app jwt", "owner", owner, "name", repoName, "app", id, "err", err)
		return
	}
	client, err := commands.NewGitHubClientWithToken(signed)
	if err != nil {
		return
	}

	installation, resp, err := client.Apps.GetRepositoryInstallation(ctx, owner, repoName)
	commands.CloseGitHub(resp)
	missing := commands.GitHubNotFound(err)
	if missing {
		err = nil
		seen = sightMissing
		return
	}
	if err != nil {
		log.Error("repository installation", "owner", owner, "name", repoName, "app", id, "err", err)
		return
	}
	if installation == nil {
		seen = sightMissing
		return
	}

	suspended := installation.SuspendedAt != nil
	if suspended {
		seen = sightSuspended
		return
	}

	installationID = installation.GetID()
	clientID = installation.GetClientID()
	missingID := installationID == 0
	missingClient := clientID == ""
	incomplete := missingID || missingClient
	if incomplete {
		err = fmt.Errorf("installation is missing")
		log.Error("repository installation", "owner", owner, "name", repoName, "app", id, "err", err)
		return
	}
	seen = sightInstalled
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
