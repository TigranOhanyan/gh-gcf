// Package build starts Cursor cloud agents for GitHub issues that are ready to build.
package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TigranOhanyan/gh-gcf/internal/source"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/repository"
)

const (
	labelTodo           = "todo"
	labelInProgress     = "in-progress"
	labelNeedsAttention = "needs-attention"

	pageSize = 100
	maxPages = 100

	defaultEndpoint = "https://api.cursor.com/v1/agents"
	maxNameLen      = 100
	maxErrorLen     = 500
)

// Options configures one build pass.
type Options struct {
	// Repo is OWNER/REPO. Empty uses the current repository.
	Repo string
	// Max is the maximum number of agents to start. Values below 1 are rejected.
	Max int
	// DryRun lists the requests that would be sent and does not change anything.
	DryRun bool
}

// Deps supplies external services. Zero values use the process environment,
// the current git repository, the GitHub API, and the Cursor Cloud Agents API.
type Deps struct {
	REST     REST
	HTTP     *http.Client
	Endpoint string
	Current  func() (Repository, error)
	Getenv   func(string) string
}

// Repository is a GitHub repository the builder can start agents in.
type Repository struct {
	Host  string
	Owner string
	Name  string
}

// URL is the repository URL passed to the Cloud Agents API.
func (r Repository) URL() string {
	host := r.Host
	if host == "" {
		host = "github.com"
	}
	return "https://" + host + "/" + r.Owner + "/" + r.Name
}

func (r Repository) slug() string {
	return r.Owner + "/" + r.Name
}

// Issue is a GitHub issue considered for a build.
type Issue struct {
	Number    int
	Title     string
	Body      string
	State     string
	Labels    []string
	CreatedAt time.Time
	IsPR      bool
}

// Run starts cloud agents for issues that are ready to build.
// A failed issue makes Run return an error after the other selected issues are processed.
// An HTTP 429 from GitHub or Cursor stops the run immediately.
// A Cursor 429 means no agent was created, so the in-progress claim is removed.
func Run(ctx context.Context, opts Options, deps Deps, stdout, stderr io.Writer) error {
	if opts.Max < 1 {
		return errors.New("max must be at least 1")
	}
	repo, err := resolveRepo(opts, deps)
	if err != nil {
		return err
	}
	model := strings.TrimSpace(deps.getenv("GCF_BUILDER_MODEL"))
	var key string
	if !opts.DryRun {
		key = strings.TrimSpace(deps.getenv("CURSOR_API_KEY"))
		if key == "" {
			return errors.New("CURSOR_API_KEY is not set; create a user API key at https://cursor.com/dashboard/api")
		}
	}

	gh, err := newGitHub(deps)
	if err != nil {
		return err
	}
	branch, err := gh.defaultBranch(ctx, repo)
	if err != nil {
		if stop := asRateLimit("GitHub", err, key); stop != nil {
			return stop
		}
		return err
	}
	issues, err := gh.list(ctx, repo)
	if err != nil {
		if stop := asRateLimit("GitHub", err, key); stop != nil {
			return stop
		}
		return err
	}
	selected := Select(issues, opts.Max)
	fmt.Fprintf(stdout, "repo: %s\ndefault branch: %s\n", repo.slug(), branch)
	if len(selected) == 0 {
		if opts.DryRun {
			fmt.Fprint(stdout, "dry-run: no issues ready to build\n")
		} else {
			fmt.Fprint(stdout, "no issues ready to build\n")
		}
		return nil
	}
	if opts.DryRun {
		return writeDryRun(stdout, endpoint(deps), repo, branch, selected, model)
	}

	cursor := newCursor(deps)
	var started, failed int
	for _, issue := range selected {
		res := startOne(ctx, gh, cursor, repo, branch, issue, model, key, stdout, stderr)
		if res.started {
			started++
		}
		if res.failed {
			failed++
		}
		if res.stop != nil {
			return res.stop
		}
	}
	fmt.Fprintf(stdout, "\n%d started, %d failed\n", started, failed)
	if failed > 0 {
		return failedError(failed)
	}
	return nil
}

func (d Deps) getenv(name string) string {
	if d.Getenv != nil {
		return d.Getenv(name)
	}
	return os.Getenv(name)
}

func endpoint(deps Deps) string {
	if deps.Endpoint != "" {
		return deps.Endpoint
	}
	return defaultEndpoint
}

func resolveRepo(opts Options, deps Deps) (Repository, error) {
	if opts.Repo != "" {
		owner, name, err := source.ParseRepo(opts.Repo)
		if err != nil {
			return Repository{}, err
		}
		return Repository{Host: "github.com", Owner: owner, Name: name}, nil
	}
	current := deps.Current
	if current == nil {
		current = currentRepository
	}
	repo, err := current()
	if err != nil {
		return Repository{}, err
	}
	if repo.Owner == "" || repo.Name == "" {
		return Repository{}, errors.New("could not determine the current repository")
	}
	return repo, nil
}

func currentRepository() (Repository, error) {
	repo, err := repository.Current()
	if err != nil {
		return Repository{}, fmt.Errorf("current repository: %w", err)
	}
	return Repository{Host: repo.Host, Owner: repo.Owner, Name: repo.Name}, nil
}

// Select returns up to max issues that are open, not pull requests, labeled todo,
// and labeled neither in-progress nor needs-attention. The result is oldest first.
func Select(issues []Issue, max int) []Issue {
	if max < 1 {
		return nil
	}
	ready := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		if issue.State != "open" || issue.IsPR || issue.Number <= 0 {
			continue
		}
		if !hasLabel(issue.Labels, labelTodo) || hasLabel(issue.Labels, labelInProgress) || hasLabel(issue.Labels, labelNeedsAttention) {
			continue
		}
		ready = append(ready, issue)
	}
	sort.SliceStable(ready, func(i, j int) bool {
		if !ready[i].CreatedAt.Equal(ready[j].CreatedAt) {
			return ready[i].CreatedAt.Before(ready[j].CreatedAt)
		}
		return ready[i].Number < ready[j].Number
	})
	if len(ready) > max {
		ready = ready[:max]
	}
	if len(ready) == 0 {
		return nil
	}
	return ready
}

func hasLabel(labels []string, name string) bool {
	for _, label := range labels {
		if label == name {
			return true
		}
	}
	return false
}

type oneResult struct {
	started bool
	failed  bool
	stop    error
}

func startOne(ctx context.Context, gh *githubAPI, cursor *cursorAPI, repo Repository, branch string, issue Issue, model, key string, stdout, stderr io.Writer) oneResult {
	fmt.Fprintf(stdout, "\n#%d %s\n", issue.Number, issue.Title)
	// Claim before calling Cursor. todo is left in place.
	if err := gh.addLabel(ctx, repo, issue.Number, labelInProgress); err != nil {
		if stop := asRateLimit("GitHub", err, key); stop != nil {
			fmt.Fprintf(stdout, "  failed: HTTP 429: %s\n", publicMessage(err, key))
			return oneResult{failed: true, stop: stop}
		}
		fmt.Fprintf(stdout, "  failed: %s\n", redact(err.Error(), key))
		return oneResult{failed: true}
	}

	agentURL, err := cursor.create(ctx, key, buildRequest(issue, repo, branch, model))
	if err != nil {
		status, message := classify(err, key)
		fmt.Fprintf(stdout, "  failed: %s\n", formatFailure(status, message))
		if status == http.StatusTooManyRequests {
			// No agent was created. Drop the claim and stop; do not mark the issue.
			if rmErr := gh.removeLabel(ctx, repo, issue.Number, labelInProgress); rmErr != nil {
				fmt.Fprintf(stderr, "warning: #%d: could not remove %s: %s\n", issue.Number, labelInProgress, redact(rmErr.Error(), key))
			}
			return oneResult{failed: true, stop: &rateLimitError{service: "Cursor", message: message}}
		}
		if markErr := markFailed(ctx, gh, repo, issue.Number, status, message, key, stderr); markErr != nil {
			if stop := asRateLimit("GitHub", markErr, key); stop != nil {
				return oneResult{failed: true, stop: stop}
			}
		}
		return oneResult{failed: true}
	}

	fmt.Fprintf(stdout, "  agent: %s\n", agentURL)
	if err := gh.comment(ctx, repo, issue.Number, successComment(agentURL)); err != nil {
		fmt.Fprintf(stderr, "warning: #%d: could not comment: %s\n", issue.Number, redact(err.Error(), key))
		if stop := asRateLimit("GitHub", err, key); stop != nil {
			return oneResult{started: true, stop: stop}
		}
	}
	return oneResult{started: true}
}

func markFailed(ctx context.Context, gh *githubAPI, repo Repository, number, status int, message, key string, stderr io.Writer) error {
	if err := gh.addLabel(ctx, repo, number, labelNeedsAttention); err != nil {
		fmt.Fprintf(stderr, "warning: #%d: could not add %s: %s\n", number, labelNeedsAttention, redact(err.Error(), key))
		if asRateLimit("GitHub", err, key) != nil {
			return err
		}
	}
	body := fmt.Sprintf("Could not start a Cursor cloud agent for this issue: %s", formatFailure(status, message))
	if err := gh.comment(ctx, repo, number, body); err != nil {
		fmt.Fprintf(stderr, "warning: #%d: could not comment: %s\n", number, redact(err.Error(), key))
		return err
	}
	return nil
}

func successComment(agentURL string) string {
	return "Started a Cursor cloud agent for this issue: " + agentURL
}

func formatFailure(status int, message string) string {
	if status > 0 {
		return fmt.Sprintf("HTTP %d: %s", status, message)
	}
	return message
}

func classify(err error, key string) (int, string) {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		message := apiErr.Message
		if message == "" {
			message = "request failed"
		}
		return apiErr.Status, truncate(redact(message, key), maxErrorLen)
	}
	return 0, truncate(redact(err.Error(), key), maxErrorLen)
}

func publicMessage(err error, key string) string {
	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) && httpErr.Message != "" {
		return truncate(redact(httpErr.Message, key), maxErrorLen)
	}
	return truncate(redact(err.Error(), key), maxErrorLen)
}

func writeDryRun(stdout io.Writer, endpoint string, repo Repository, branch string, issues []Issue, model string) error {
	noun := "issues"
	if len(issues) == 1 {
		noun = "issue"
	}
	fmt.Fprintf(stdout, "dry-run: %d %s, no changes made\n", len(issues), noun)
	for _, issue := range issues {
		fmt.Fprintf(stdout, "\n#%d %s\n", issue.Number, issue.Title)
		fmt.Fprintf(stdout, "POST %s\n", endpoint)
		body, err := encodeJSONIndent(buildRequest(issue, repo, branch, model))
		if err != nil {
			return err
		}
		if _, err := stdout.Write(body); err != nil {
			return err
		}
	}
	return nil
}

func buildRequest(issue Issue, repo Repository, branch, model string) agentRequest {
	req := agentRequest{
		Prompt:       agentPrompt{Text: promptText(issue)},
		Repos:        []agentRepo{{URL: repo.URL(), StartingRef: branch}},
		AutoCreatePR: true,
		Name:         agentName(issue.Number, issue.Title),
	}
	if model != "" {
		req.Model = &agentModel{ID: model}
	}
	return req
}

func promptText(issue Issue) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Implement GitHub issue #%d in this repository.\n\n", issue.Number)
	b.WriteString("The issue contents are included below. Do not fetch the issue from GitHub.\n\n")
	fmt.Fprintf(&b, "Title: %s\n\n", issue.Title)
	fmt.Fprintf(&b, "Issue body:\n%s\n\n", issue.Body)
	b.WriteString("Rules:\n")
	b.WriteString("- Implement only what the issue asks.\n")
	b.WriteString("- Keep the change small and follow the repository's conventions.\n")
	b.WriteString("- Run the repository's tests or checks if any exist.\n")
	b.WriteString("- Never edit `.factory/` or `.github/workflows/`.\n")
	fmt.Fprintf(&b, "- The pull request body must say what changed, how it was checked, and include `Closes #%d`.\n", issue.Number)
	return b.String()
}

func agentName(number int, title string) string {
	title = strings.Join(strings.Fields(title), " ")
	var name string
	if title == "" {
		name = fmt.Sprintf("gcf build #%d", number)
	} else {
		name = fmt.Sprintf("gcf build #%d: %s", number, title)
	}
	return truncate(name, maxNameLen)
}

func truncate(s string, max int) string {
	if max < 1 || s == "" || utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

func redact(s, key string) string {
	key = strings.TrimSpace(key)
	if len(key) < 8 || s == "" || !strings.Contains(s, key) {
		return s
	}
	return strings.ReplaceAll(s, key, "[redacted]")
}

func failedError(n int) error {
	if n == 1 {
		return errors.New("1 issue failed")
	}
	return fmt.Errorf("%d issues failed", n)
}

type rateLimitError struct {
	service string
	message string
}

func (e *rateLimitError) Error() string {
	if e.message == "" {
		return fmt.Sprintf("stopped: %s API rate limit (HTTP 429); no further issues were started", e.service)
	}
	return fmt.Sprintf("stopped: %s API rate limit (HTTP 429): %s; no further issues were started", e.service, e.message)
}

func asRateLimit(service string, err error, key string) error {
	if err == nil {
		return nil
	}
	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusTooManyRequests {
		return &rateLimitError{service: service, message: truncate(redact(httpErr.Message, key), maxErrorLen)}
	}
	var apiErr *apiError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusTooManyRequests {
		return &rateLimitError{service: service, message: truncate(redact(apiErr.Message, key), maxErrorLen)}
	}
	return nil
}
