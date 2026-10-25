package build

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// REST is the subset of go-gh's REST client the builder uses.
type REST interface {
	RequestWithContext(ctx context.Context, method, path string, body io.Reader) (*http.Response, error)
}

type githubAPI struct {
	rest REST
}

func newGitHub(deps Deps) (*githubAPI, error) {
	if deps.REST != nil {
		return &githubAPI{rest: deps.REST}, nil
	}
	client, err := api.NewRESTClient(api.ClientOptions{
		Timeout: 2 * time.Minute,
		Headers: map[string]string{
			"Accept":               "application/vnd.github+json",
			"X-GitHub-Api-Version": "2022-11-28",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("GitHub client (gh auth login): %w", err)
	}
	return &githubAPI{rest: client}, nil
}

func (g *githubAPI) defaultBranch(ctx context.Context, repo Repository) (string, error) {
	var payload struct {
		DefaultBranch string `json:"default_branch"`
	}
	path := repoPath(repo)
	if err := getJSON(ctx, g.rest, path, &payload); err != nil {
		if isNotFound(err) {
			return "", fmt.Errorf("repository %s not found", repo.slug())
		}
		return "", fmt.Errorf("look up default branch of %s: %w", repo.slug(), err)
	}
	if payload.DefaultBranch == "" {
		return "", fmt.Errorf("repository %s has no default branch", repo.slug())
	}
	return payload.DefaultBranch, nil
}

func (g *githubAPI) list(ctx context.Context, repo Repository) ([]Issue, error) {
	var all []Issue
	seen := map[int]struct{}{}
	for page := 1; page <= maxPages; page++ {
		batch, err := g.listPage(ctx, repo, page)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			return all, nil
		}
		fresh := 0
		for _, issue := range batch {
			if issue.Number <= 0 {
				continue
			}
			if _, ok := seen[issue.Number]; ok {
				continue
			}
			seen[issue.Number] = struct{}{}
			all = append(all, issue)
			fresh++
		}
		if fresh == 0 || len(batch) < pageSize {
			return all, nil
		}
		if page == maxPages {
			return nil, fmt.Errorf("list issues in %s: more than %d open issues labeled %s", repo.slug(), maxPages*pageSize, labelTodo)
		}
	}
	return all, nil
}

func (g *githubAPI) listPage(ctx context.Context, repo Repository, page int) ([]Issue, error) {
	var payload []issuePayload
	path := issuesPath(repo, page)
	if err := getJSON(ctx, g.rest, path, &payload); err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("repository %s not found", repo.slug())
		}
		return nil, fmt.Errorf("list issues in %s: %w", repo.slug(), err)
	}
	issues := make([]Issue, 0, len(payload))
	for _, item := range payload {
		issues = append(issues, item.issue())
	}
	return issues, nil
}

// addLabel adds one label. It does not remove labels that are already present.
func (g *githubAPI) addLabel(ctx context.Context, repo Repository, number int, label string) error {
	path := fmt.Sprintf("%s/issues/%d/labels", repoPath(repo), number)
	err := postJSON(ctx, g.rest, path, labelsPayload{Labels: []string{label}})
	if err != nil {
		return fmt.Errorf("add label %s to %s#%d: %w", label, repo.slug(), number, err)
	}
	return nil
}

func (g *githubAPI) comment(ctx context.Context, repo Repository, number int, body string) error {
	path := fmt.Sprintf("%s/issues/%d/comments", repoPath(repo), number)
	err := postJSON(ctx, g.rest, path, commentPayload{Body: body})
	if err != nil {
		return fmt.Errorf("comment on %s#%d: %w", repo.slug(), number, err)
	}
	return nil
}

type issuePayload struct {
	Number      int             `json:"number"`
	Title       string          `json:"title"`
	Body        *string         `json:"body"`
	State       string          `json:"state"`
	CreatedAt   time.Time       `json:"created_at"`
	Labels      []labelPayload  `json:"labels"`
	PullRequest json.RawMessage `json:"pull_request"`
}

type labelPayload struct {
	Name string `json:"name"`
}

type labelsPayload struct {
	Labels []string `json:"labels"`
}

type commentPayload struct {
	Body string `json:"body"`
}

func (p issuePayload) issue() Issue {
	labels := make([]string, 0, len(p.Labels))
	for _, label := range p.Labels {
		if label.Name != "" {
			labels = append(labels, label.Name)
		}
	}
	body := ""
	if p.Body != nil {
		body = *p.Body
	}
	return Issue{
		Number:    p.Number,
		Title:     p.Title,
		Body:      body,
		State:     p.State,
		Labels:    labels,
		CreatedAt: p.CreatedAt,
		IsPR:      isPullRequest(p.PullRequest),
	}
}

func isPullRequest(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) > 0 && !bytes.Equal(raw, []byte("null"))
}

func repoPath(repo Repository) string {
	return "repos/" + url.PathEscape(repo.Owner) + "/" + url.PathEscape(repo.Name)
}

func issuesPath(repo Repository, page int) string {
	query := url.Values{}
	query.Set("state", "open")
	query.Set("labels", labelTodo)
	query.Set("sort", "created")
	query.Set("direction", "asc")
	query.Set("per_page", strconv.Itoa(pageSize))
	query.Set("page", strconv.Itoa(page))
	return repoPath(repo) + "/issues?" + query.Encode()
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

func postJSON(ctx context.Context, client REST, path string, payload any) error {
	body, err := encodeJSON(payload)
	if err != nil {
		return err
	}
	resp, err := client.RequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	return err
}

func isNotFound(err error) bool {
	var httpErr *api.HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound
}
