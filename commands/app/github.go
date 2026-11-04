package app

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
	"time"

	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

const keyPrefix = "GITHUB_APP_"
const jwtHeader = `{"alg":"RS256","typ":"JWT"}`

type appClaims struct {
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
	Iss string `json:"iss"`
}

func (step appStep) lookUp(ctx context.Context, owner string, repoName string, id int64) (seen sight, clientID string, installationID int64, err error) {
	key, err := loadAppKey(id)
	if err != nil {
		return
	}

	now := time.Now()
	signed, err := signAppJWT(id, key, now)
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
	key, err = parseAppKey(pemText)
	if err != nil {
		log.Error("github app", "app", id, "err", err)
	}
	return
}

func signAppJWT(appID int64, key *rsa.PrivateKey, now time.Time) (signed string, err error) {
	issuedAt := now.Add(-time.Minute).Unix()
	expiresAt := now.Add(8 * time.Minute).Unix()
	appIDText := strconv.FormatInt(appID, 10)
	claims := appClaims{
		Iat: issuedAt,
		Exp: expiresAt,
		Iss: appIDText,
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(jwtHeader))
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := header + "." + encoded
	sum := sha256.Sum256([]byte(signingInput))
	digest := sum[:]
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest)
	if err != nil {
		return
	}
	signed = signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
	return
}

func parseAppKey(pemText string) (key *rsa.PrivateKey, err error) {
	block, _ := pem.Decode([]byte(pemText))
	if block == nil {
		err = fmt.Errorf("github app private key is not pem")
		return
	}

	parsed, parseErr := x509.ParsePKCS1PrivateKey(block.Bytes)
	if parseErr == nil {
		key = parsed
		return
	}
	generic, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
	if parseErr != nil {
		err = fmt.Errorf("github app private key is not an rsa key")
		return
	}
	rsaKey, isRSA := generic.(*rsa.PrivateKey)
	if !isRSA {
		err = fmt.Errorf("github app private key is not an rsa key")
		return
	}
	key = rsaKey
	return
}

func finishTx(tx *sql.Tx, workErr error) (err error) {
	failed := workErr != nil
	if failed {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Error("rollback", "err", rollbackErr)
		}
		err = workErr
		return
	}
	err = tx.Commit()
	if err != nil {
		log.Error("commit", "err", err)
	}
	return
}
