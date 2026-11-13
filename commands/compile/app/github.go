package app

import (
	"context"
	"crypto/rsa"
	"fmt"
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/TigranOhanyan/gh-gcf/logging"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

const keyPrefix = "FACTORY_GITHUB_APP_"

func (step appStep) lookUp(ctx context.Context, owner string, repoName string, id int64) (seen sight, clientID string, installationID int64, err error) {
	key, err := loadAppKey(id)
	if err != nil {
		return
	}

	now := time.Now()
	signed, err := commands.SignAppJWT(id, key, now)
	if err != nil {
		logging.From(ctx).Error("app jwt", zap.Any("owner", owner), zap.Any("name", repoName), zap.Any("app", id), zap.Error(err))
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
		logging.From(ctx).Error("repository installation", zap.Any("owner", owner), zap.Any("name", repoName), zap.Any("app", id), zap.Error(err))
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
		logging.From(ctx).Error("repository installation", zap.Any("owner", owner), zap.Any("name", repoName), zap.Any("app", id), zap.Error(err))
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
		zap.L().Error("github app", zap.Any("app", id), zap.Error(err))
		return
	}
	key, err = commands.ParseAppKey(pemText)
	if err != nil {
		zap.L().Error("github app", zap.Any("app", id), zap.Error(err))
	}
	return
}
