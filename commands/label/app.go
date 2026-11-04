package label

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
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
const jwtHeader = `{"alg":"RS256","typ":"JWT"}`

type appAuth struct {
	id  int64
	key *rsa.PrivateKey
}

type appClaims struct {
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
	Iss string `json:"iss"`
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
	key, err := parseAppKey(pemText)
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
	signed, err := signAppJWT(app.id, app.key, now)
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
