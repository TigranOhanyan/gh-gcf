package commands

import (
	"context"
	"crypto/rsa"
	"fmt"
	"os"
	"strconv"
	"time"

	"charm.land/log/v2"
)

const appKeyPrefix = "FACTORY_GITHUB_APP_"

// InstallationToken mints an installation token for appID on owner/repo.
// A missing key, a key that is not a PEM, a missing installation, or an empty token is an error.
func InstallationToken(ctx context.Context, appID int64, owner string, repo string) (token string, err error) {
	key, err := loadAppKey(appID)
	if err != nil {
		return
	}
	now := time.Now()
	signed, err := SignAppJWT(appID, key, now)
	if err != nil {
		log.Error("app jwt", "app", appID, "err", err)
		return
	}
	appClient, err := NewGitHubClientWithToken(signed)
	if err != nil {
		return
	}
	installation, resp, err := appClient.Apps.GetRepositoryInstallation(ctx, owner, repo)
	CloseGitHub(resp)
	if err != nil {
		log.Error("repository installation", "owner", owner, "name", repo, "app", appID, "err", err)
		return
	}
	missingInstallation := installation == nil
	if missingInstallation {
		err = fmt.Errorf("installation is missing")
		log.Error("repository installation", "owner", owner, "name", repo, "app", appID, "err", err)
		return
	}
	installationID := installation.GetID()
	missingID := installationID == 0
	if missingID {
		err = fmt.Errorf("installation is missing")
		log.Error("repository installation", "owner", owner, "name", repo, "app", appID, "err", err)
		return
	}
	issued, tokenResp, err := appClient.Apps.CreateInstallationToken(ctx, installationID, nil)
	CloseGitHub(tokenResp)
	if err != nil {
		log.Error("installation token", "app", appID, "installation", installationID, "err", err)
		return
	}
	if issued != nil {
		token = issued.GetToken()
	}
	missingToken := token == ""
	if missingToken {
		err = fmt.Errorf("installation token is empty")
		log.Error("installation token", "app", appID, "installation", installationID, "err", err)
	}
	return
}

func loadAppKey(id int64) (key *rsa.PrivateKey, err error) {
	name := appKeyPrefix + strconv.FormatInt(id, 10)
	pemText := os.Getenv(name)
	missing := pemText == ""
	if missing {
		err = fmt.Errorf("github app key is missing")
		log.Error("github app", "app", id, "err", err)
		return
	}
	key, err = ParseAppKey(pemText)
	if err != nil {
		log.Error("github app", "app", id, "err", err)
	}
	return
}
