package commands

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
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"
)

const githubAPIURL = "GITHUB_API_URL"

// UserGitHubClient returns a client for GH_TOKEN, then GITHUB_TOKEN, then `gh auth token`.
func UserGitHubClient(ctx context.Context) (client *github.Client, err error) {
	token, err := userGitHubToken(ctx)
	if err != nil {
		return
	}
	client, err = NewGitHubClientWithToken(token)
	return
}

func userGitHubToken(ctx context.Context) (token string, err error) {
	token = os.Getenv("GH_TOKEN")
	if token != "" {
		return
	}
	token = os.Getenv("GITHUB_TOKEN")
	if token != "" {
		return
	}
	token, err = ghAuthToken(ctx)
	return
}

func ghAuthToken(ctx context.Context) (token string, err error) {
	exe := os.Getenv("GH_PATH")
	if exe == "" {
		exe = "gh"
	}
	cmd := exec.CommandContext(ctx, exe, "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		log.Error("gh auth token", "err", err)
		err = fmt.Errorf("github token is missing")
		return
	}
	token = strings.TrimSpace(string(output))
	missing := token == ""
	if missing {
		err = fmt.Errorf("github token is missing")
		log.Error("gh auth token", "err", err)
	}
	return
}

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

const jwtHeader = `{"alg":"RS256","typ":"JWT"}`

type appClaims struct {
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
	Iss string `json:"iss"`
}

// SignAppJWT signs a GitHub App JWT for appID with key.
// The token is valid for about eight minutes from now.
func SignAppJWT(appID int64, key *rsa.PrivateKey, now time.Time) (signed string, err error) {
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

// ParseAppKey reads an RSA private key from PEM text.
// PKCS1 and PKCS8 are both accepted.
func ParseAppKey(pemText string) (key *rsa.PrivateKey, err error) {
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

func withTrailingSlash(base string) (url string) {
	url = base
	missing := !strings.HasSuffix(url, "/")
	if missing {
		url = url + "/"
	}
	return
}
