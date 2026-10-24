// Package source downloads a factory repository tarball with the user's gh credentials.
package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// DefaultRepo is the factory repository used when --source is omitted.
const DefaultRepo = "TigranOhanyan/github-cursor-factory"

const (
	maxArchiveBytes = 64 << 20
	requestTimeout  = 2 * time.Minute
)

// Fetched is a factory archive pinned to a commit.
// ReleaseTag is set when ref is "latest" and names the release that was selected.
type Fetched struct {
	Commit     string
	ReleaseTag string
	Archive    []byte
}

// REST is the subset of go-gh's REST client used to download a factory.
type REST interface {
	RequestWithContext(ctx context.Context, method, path string, body io.Reader) (*http.Response, error)
}

// GitHub downloads factories through the GitHub API using the user's gh login.
type GitHub struct{}

// Fetch resolves ref and downloads the repository tarball.
// The ref "latest" selects the newest published release and fails when the repository has none.
func (GitHub) Fetch(ctx context.Context, repo, ref string) (Fetched, error) {
	client, err := api.NewRESTClient(api.ClientOptions{
		Timeout: requestTimeout,
		Headers: map[string]string{
			"Accept":               "application/vnd.github+json",
			"X-GitHub-Api-Version": "2022-11-28",
		},
	})
	if err != nil {
		return Fetched{}, fmt.Errorf("GitHub client (gh auth login): %w", err)
	}
	return Fetch(ctx, client, repo, ref)
}

// Fetch resolves ref against client and downloads the gzip tarball for that ref.
func Fetch(ctx context.Context, client REST, repo, ref string) (Fetched, error) {
	owner, name, err := ParseRepo(repo)
	if err != nil {
		return Fetched{}, err
	}
	if err := ValidateRef(ref); err != nil {
		return Fetched{}, err
	}

	downloadRef := ref
	var releaseTag string
	if ref == "latest" {
		tag, err := latestReleaseTag(ctx, client, owner, name)
		if err != nil {
			return Fetched{}, err
		}
		downloadRef = tag
		releaseTag = tag
	}

	commit, err := resolveCommit(ctx, client, owner, name, downloadRef)
	if err != nil {
		return Fetched{}, err
	}
	archive, err := downloadArchive(ctx, client, owner, name, downloadRef)
	if err != nil {
		return Fetched{}, err
	}
	return Fetched{Commit: commit, ReleaseTag: releaseTag, Archive: archive}, nil
}

// ParseRepo splits "owner/repo".
func ParseRepo(repo string) (owner, name string, err error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") ||
		!validRepoPart(owner) || !validRepoPart(name) {
		return "", "", fmt.Errorf("source must be OWNER/REPO, got %q", repo)
	}
	return owner, name, nil
}

func validRepoPart(part string) bool {
	if part == "." || part == ".." {
		return false
	}
	return !strings.ContainsAny(part, " \t\r\n?#%\\")
}

// ValidateRef accepts a tag, branch, commit SHA, or the literal "latest".
func ValidateRef(ref string) error {
	if ref == "" {
		return errors.New("ref is empty")
	}
	if strings.ContainsAny(ref, " \t\r\n?#\\") || strings.HasPrefix(ref, "/") || strings.HasSuffix(ref, "/") {
		return fmt.Errorf("invalid ref %q", ref)
	}
	for _, seg := range strings.Split(ref, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return fmt.Errorf("invalid ref %q", ref)
		}
	}
	return nil
}

func latestReleaseTag(ctx context.Context, client REST, owner, name string) (string, error) {
	var rel struct {
		TagName string `json:"tag_name"`
	}
	err := getJSON(ctx, client, apiPath(owner, name, "releases", "latest"), &rel)
	if err == nil {
		if rel.TagName == "" {
			return "", fmt.Errorf("%s/%s latest release has no tag name", owner, name)
		}
		if err := ValidateRef(rel.TagName); err != nil {
			return "", fmt.Errorf("%s/%s latest release tag: %w", owner, name, err)
		}
		return rel.TagName, nil
	}
	if !isNotFound(err) {
		return "", fmt.Errorf("look up latest release of %s/%s: %w", owner, name, err)
	}
	if repoErr := getJSON(ctx, client, apiPath(owner, name), &struct{}{}); isNotFound(repoErr) {
		return "", fmt.Errorf("repository %s/%s not found", owner, name)
	} else if repoErr != nil {
		return "", fmt.Errorf("look up latest release of %s/%s: %w", owner, name, err)
	}
	return "", fmt.Errorf("%s/%s has no releases yet; pass a tag, branch, or commit SHA", owner, name)
}

func resolveCommit(ctx context.Context, client REST, owner, name, ref string) (string, error) {
	var commit struct {
		SHA string `json:"sha"`
	}
	path := apiPath(owner, name, "commits", url.PathEscape(ref))
	if err := getJSON(ctx, client, path, &commit); err != nil {
		if isNotFound(err) {
			return "", fmt.Errorf("ref %q not found in %s/%s", ref, owner, name)
		}
		return "", fmt.Errorf("resolve %q in %s/%s: %w", ref, owner, name, err)
	}
	sha := strings.ToLower(commit.SHA)
	if len(sha) != 40 || strings.Trim(sha, "0123456789abcdef") != "" {
		return "", fmt.Errorf("ref %q in %s/%s resolved to %q, want a 40-character commit SHA", ref, owner, name, commit.SHA)
	}
	return sha, nil
}

func downloadArchive(ctx context.Context, client REST, owner, name, ref string) ([]byte, error) {
	path := apiPath(owner, name, "tarball", url.PathEscape(ref))
	resp, err := client.RequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("archive for %s/%s at %q not found", owner, name, ref)
		}
		return nil, fmt.Errorf("download %s/%s at %q: %w", owner, name, ref, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveBytes+1))
	if err != nil {
		return nil, fmt.Errorf("download %s/%s at %q: %w", owner, name, ref, err)
	}
	if len(body) > maxArchiveBytes {
		return nil, fmt.Errorf("archive for %s/%s at %q exceeds %d bytes", owner, name, ref, maxArchiveBytes)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("archive for %s/%s at %q is empty", owner, name, ref)
	}
	return body, nil
}

func apiPath(owner, name string, tail ...string) string {
	parts := make([]string, 0, 2+len(tail))
	parts = append(parts, "repos", url.PathEscape(owner), url.PathEscape(name))
	parts = append(parts, tail...)
	return strings.Join(parts, "/")
}

func getJSON(ctx context.Context, client REST, path string, dest any) error {
	resp, err := client.RequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func isNotFound(err error) bool {
	var httpErr *api.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound
}
