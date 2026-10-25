package build

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

const testKey = "crsr_test_key_abcdef"

func TestSelectFiltersAndOrders(t *testing.T) {
	older := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	issues := []Issue{
		{Number: 8, Title: "no todo", State: "open", CreatedAt: older, Labels: []string{"bug"}},
		{Number: 4, Title: "pr", State: "open", CreatedAt: older, Labels: []string{labelTodo}, IsPR: true},
		{Number: 2, Title: "busy", State: "open", CreatedAt: older, Labels: []string{labelTodo, labelInProgress}},
		{Number: 6, Title: "attn", State: "open", CreatedAt: older, Labels: []string{labelTodo, labelNeedsAttention}},
		{Number: 7, Title: "closed", State: "closed", CreatedAt: older, Labels: []string{labelTodo}},
		{Number: 1, Title: "unstated", CreatedAt: older, Labels: []string{labelTodo}},
		{Number: 9, Title: "newer", State: "open", CreatedAt: newer, Labels: []string{labelTodo}},
		{Number: 11, Title: "same later", State: "open", CreatedAt: older, Labels: []string{labelTodo}},
		{Number: 3, Title: "older", State: "open", CreatedAt: older, Labels: []string{"bug", labelTodo}},
		{Number: 10, Title: "same earlier", State: "open", CreatedAt: older, Labels: []string{labelTodo}},
	}
	got := numbers(Select(issues, 4))
	want := []int{3, 10, 11, 9}
	if strings.Join(ints(got), ",") != strings.Join(ints(want), ",") {
		t.Fatalf("select: got %v want %v", got, want)
	}
	if got := numbers(Select(issues, 1)); len(got) != 1 || got[0] != 3 {
		t.Fatalf("max 1: %v", got)
	}
	if Select(issues, 0) != nil {
		t.Fatal("max 0 returned issues")
	}
	if issues[0].Title != "no todo" {
		t.Fatal("Select mutated the input")
	}
}

func TestPromptAndName(t *testing.T) {
	issue := Issue{Number: 12, Title: "Fix the widget", Body: "The widget overflows.\n"}
	text := promptText(issue)
	for _, want := range []string{
		"Implement GitHub issue #12 in this repository.",
		"Title: Fix the widget",
		"Issue body:\nThe widget overflows.\n",
		"Implement only what the issue asks.",
		"Keep the change small and follow the repository's conventions.",
		"Run the repository's tests or checks if any exist.",
		"Never edit `.factory/` or `.github/workflows/`.",
		"The pull request body must say what changed, how it was checked, and include `Closes #12`.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("prompt missing %q\n%s", want, text)
		}
	}
	if agentName(12, "Fix the widget") != "gcf build #12: Fix the widget" {
		t.Fatal(agentName(12, "Fix the widget"))
	}
	if agentName(12, "  Fix\nthe  widget ") != "gcf build #12: Fix the widget" {
		t.Fatal(agentName(12, "  Fix\nthe  widget "))
	}
	if agentName(12, " ") != "gcf build #12" {
		t.Fatal(agentName(12, " "))
	}
	long := agentName(7, strings.Repeat("é", 200))
	if countRunes(long) != maxNameLen {
		t.Fatalf("name length %d", countRunes(long))
	}
	if !strings.HasPrefix(long, "gcf build #7: ") {
		t.Fatalf("name %q", long)
	}
}

func TestCursorErrorMessage(t *testing.T) {
	nested := cursorErrorMessage([]byte(`{"error":{"code":"validation_error","message":"model rejected"}}`))
	if nested != "model rejected" {
		t.Fatalf("nested: %q", nested)
	}
	flat := cursorErrorMessage([]byte(`{"code":"error","message":"Invalid User API Key"}`))
	if flat != "Invalid User API Key" {
		t.Fatalf("flat: %q", flat)
	}
	if cursorErrorMessage(nil) != "request failed" {
		t.Fatalf("empty: %q", cursorErrorMessage(nil))
	}
}

func TestRunSelectionOrderMaxAndClaim(t *testing.T) {
	res := runAgents(t, mixedIssues(t), Options{Repo: "acme/widgets", Max: 2}, envWith(testKey, ""), "develop", nil)
	if res.err != nil {
		t.Fatal(res.err)
	}
	if len(res.rec.bodies) != 2 {
		t.Fatalf("cursor calls: %d\n%s", len(res.rec.bodies), res.stdout)
	}
	if !bytes.Contains(res.rec.bodies[0], []byte("BODY_OLDER")) || bytes.Contains(res.rec.bodies[0], []byte("BODY_NEWER")) {
		t.Fatalf("first body:\n%s", res.rec.bodies[0])
	}
	if !bytes.Contains(res.rec.bodies[1], []byte("BODY_NEWER")) {
		t.Fatalf("second body:\n%s", res.rec.bodies[1])
	}
	joined := bytes.Join(res.rec.bodies, nil)
	for _, skipped := range []string{"BODY_PR", "BODY_BUSY", "BODY_ATTN", "BODY_CLOSED", "BODY_OTHER", "BODY_NEWEST"} {
		if bytes.Contains(joined, []byte(skipped)) {
			t.Fatalf("request included %s\n%s", skipped, joined)
		}
	}
	wantEvents := []string{
		"GET repo",
		"GET issues",
		"POST labels #15 in-progress",
		"POST cursor",
		"POST comment #15",
		"POST labels #16 in-progress",
		"POST cursor",
		"POST comment #16",
	}
	if got := strings.Join(res.rec.events(), "\n"); got != strings.Join(wantEvents, "\n") {
		t.Fatalf("events:\n%s", got)
	}
	if res.rec.auths[0] != "Bearer "+testKey {
		t.Fatalf("authorization %q", res.rec.auths[0])
	}
	if strings.Contains(res.rec.auths[0], "\n") || bytes.Contains(joined, []byte(testKey)) {
		t.Fatal("API key leaked into the request")
	}
	if strings.Contains(res.stdout, testKey) || strings.Contains(res.stderr, testKey) {
		t.Fatal("API key leaked into output")
	}
	for _, call := range res.rest.snapshot() {
		if call.Method != http.MethodGet && call.Method != http.MethodPost {
			t.Fatalf("unexpected %s %s", call.Method, call.Path)
		}
		if strings.Contains(call.Body, `"todo"`) {
			t.Fatalf("todo label was sent: %s", call.Body)
		}
	}
	gets := res.rest.issueQueries()
	if len(gets) != 1 {
		t.Fatalf("issue queries: %v", gets)
	}
	for _, part := range []string{"state=open", "labels=todo", "sort=created", "direction=asc", "per_page=100"} {
		if !strings.Contains(gets[0], part) {
			t.Fatalf("query %s missing %s", gets[0], part)
		}
	}
	if !strings.Contains(res.stdout, "repo: acme/widgets") || !strings.Contains(res.stdout, "default branch: develop") {
		t.Fatalf("stdout:\n%s", res.stdout)
	}
	if !strings.Contains(res.stdout, "https://cursor.com/agents/bc-1") || !strings.Contains(res.stdout, "\n2 started, 0 failed\n") {
		t.Fatalf("stdout:\n%s", res.stdout)
	}
	comments := res.rest.comments()
	if len(comments) != 2 || !strings.Contains(comments[0], "https://cursor.com/agents/bc-1") {
		t.Fatalf("comments: %#v", comments)
	}
}

func TestRunRequestJSON(t *testing.T) {
	issues := oneIssue(t, 15, "older", "BODY_OLDER")
	t.Run("without model", func(t *testing.T) {
		res := runAgents(t, issues, Options{Repo: "acme/widgets", Max: 1}, envWith("  "+testKey+"\n", " \n"), "develop", nil)
		if res.err != nil {
			t.Fatal(res.err)
		}
		got := decodeRequest(t, res.rec.bodies[0])
		if got.Model != nil {
			t.Fatalf("model %#v", got.Model)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(res.rec.bodies[0], &raw); err != nil {
			t.Fatal(err)
		}
		if _, ok := raw["model"]; ok {
			t.Fatalf("model key present: %s", res.rec.bodies[0])
		}
		assertRequest(t, got, 15, "older", "BODY_OLDER", "develop")
		if res.rec.auths[0] != "Bearer "+testKey {
			t.Fatalf("authorization %q", res.rec.auths[0])
		}
		if res.rec.types[0] != "application/json" {
			t.Fatalf("content-type %s", res.rec.types[0])
		}
	})
	t.Run("with model", func(t *testing.T) {
		res := runAgents(t, issues, Options{Repo: "acme/widgets", Max: 1}, envWith(testKey, " composer-2\n"), "develop", nil)
		if res.err != nil {
			t.Fatal(res.err)
		}
		got := decodeRequest(t, res.rec.bodies[0])
		if got.Model == nil || got.Model.ID != "composer-2" {
			t.Fatalf("model %#v", got.Model)
		}
		assertRequest(t, got, 15, "older", "BODY_OLDER", "develop")
	})
}

func TestRunMissingAPIKey(t *testing.T) {
	for _, key := range []string{"", " \n", "\t"} {
		rest := &fakeREST{branch: "main", pages: map[int][]byte{1: oneIssue(t, 1, "one", "body")}}
		var called atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called.Store(true)
			http.Error(w, "called", http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)
		var stdout, stderr bytes.Buffer
		err := Run(context.Background(), Options{Repo: "acme/widgets", Max: 1}, Deps{
			REST:     rest,
			Endpoint: srv.URL + "/v1/agents",
			Getenv: func(name string) string {
				if name == "CURSOR_API_KEY" {
					return key
				}
				return ""
			},
		}, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), "CURSOR_API_KEY is not set") {
			t.Fatalf("key %q: %v", key, err)
		}
		if strings.Contains(err.Error(), testKey) || called.Load() || len(rest.snapshot()) != 0 {
			t.Fatalf("key %q leaked or called APIs: err %v calls %d cursor %v", key, err, len(rest.snapshot()), called.Load())
		}
	}

	t.Setenv("CURSOR_API_KEY", "\n")
	rest := &fakeREST{}
	err := Run(context.Background(), Options{Repo: "acme/widgets", Max: 1}, Deps{REST: rest}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "CURSOR_API_KEY is not set") {
		t.Fatalf("environment key: %v", err)
	}
	if len(rest.snapshot()) != 0 {
		t.Fatal("environment key still called GitHub")
	}
}

func TestRunCursorErrorContinues(t *testing.T) {
	res := runAgents(t, twoIssues(t), Options{Repo: "acme/widgets", Max: 2}, envWith(testKey, ""), "main", func(w http.ResponseWriter, r *http.Request, body []byte, n int) {
		if bytes.Contains(body, []byte(testKey)) {
			t.Errorf("request body contains the API key")
		}
		if n == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error":{"code":"validation_error","message":"model rejected %s"}}`, testKey)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"agent":{"url":"https://cursor.com/agents/bc-second"},"run":{"id":"run-2"}}`)
	})
	if res.err == nil || res.err.Error() != "1 issue failed" {
		t.Fatalf("err %v\nstdout:\n%s", res.err, res.stdout)
	}
	for _, text := range []string{res.err.Error(), res.stdout, res.stderr} {
		if strings.Contains(text, testKey) {
			t.Fatalf("API key leaked: %s", text)
		}
	}
	wantEvents := []string{
		"GET repo",
		"GET issues",
		"POST labels #1 in-progress",
		"POST cursor",
		"POST labels #1 needs-attention",
		"POST comment #1",
		"POST labels #2 in-progress",
		"POST cursor",
		"POST comment #2",
	}
	if got := strings.Join(res.rec.events(), "\n"); got != strings.Join(wantEvents, "\n") {
		t.Fatalf("events:\n%s", got)
	}
	comments := res.rest.comments()
	if len(comments) != 2 {
		t.Fatalf("comments: %#v", comments)
	}
	if !strings.Contains(comments[0], "HTTP 400") || !strings.Contains(comments[0], "model rejected") || strings.Contains(comments[0], testKey) {
		t.Fatalf("failure comment: %s", comments[0])
	}
	if !strings.Contains(comments[0], "[redacted]") {
		t.Fatalf("failure comment did not redact the key: %s", comments[0])
	}
	if !strings.Contains(comments[1], "https://cursor.com/agents/bc-second") {
		t.Fatalf("success comment: %s", comments[1])
	}
	if strings.Contains(res.stdout, testKey) || !strings.Contains(res.stdout, "https://cursor.com/agents/bc-second") || !strings.Contains(res.stdout, "1 started, 1 failed") {
		t.Fatalf("stdout:\n%s", res.stdout)
	}
	if !strings.Contains(res.stdout, "HTTP 400") || !strings.Contains(res.stdout, "model rejected") {
		t.Fatalf("stdout:\n%s", res.stdout)
	}

	t.Run("flat unauthorized body", func(t *testing.T) {
		res := runAgents(t, oneIssue(t, 4, "auth", "BODY_AUTH"), Options{Repo: "acme/widgets", Max: 1}, envWith(testKey, ""), "main", func(w http.ResponseWriter, r *http.Request, body []byte, n int) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `{"code":"error","message":"Invalid User API Key"}`)
		})
		if res.err == nil || res.err.Error() != "1 issue failed" {
			t.Fatal(res.err)
		}
		comments := res.rest.comments()
		if len(comments) != 1 || !strings.Contains(comments[0], "HTTP 401") || !strings.Contains(comments[0], "Invalid User API Key") {
			t.Fatalf("comment: %#v", comments)
		}
		if !strings.Contains(strings.Join(res.rec.events(), "\n"), "POST labels #4 needs-attention") {
			t.Fatalf("events:\n%s", strings.Join(res.rec.events(), "\n"))
		}
	})
}

func TestRunDryRunMakesNoChanges(t *testing.T) {
	var hits atomic.Int32
	rest := &fakeREST{branch: "develop", pages: map[int][]byte{1: mixedIssues(t)}}
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), Options{Repo: "acme/widgets", Max: 1, DryRun: true}, Deps{
		REST: rest,
		HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			hits.Add(1)
			return nil, errors.New("network disabled")
		})},
		Getenv: func(name string) string {
			switch name {
			case "CURSOR_API_KEY":
				return "  " + testKey + "\n"
			default:
				return ""
			}
		},
	}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 0 {
		t.Fatalf("cursor called %d times", hits.Load())
	}
	for _, call := range rest.snapshot() {
		if call.Method != http.MethodGet {
			t.Fatalf("dry-run write: %s %s %s", call.Method, call.Path, call.Body)
		}
	}
	out := stdout.String()
	if strings.Contains(out, testKey) || strings.Contains(stderr.String(), testKey) {
		t.Fatal("dry-run printed the API key")
	}
	if !strings.Contains(out, "dry-run: 1 issue, no changes made") || !strings.Contains(out, "POST https://api.cursor.com/v1/agents\n") {
		t.Fatalf("stdout:\n%s", out)
	}
	if strings.Contains(out, "BODY_NEWER") || strings.Contains(out, "BODY_NEWEST") || strings.Contains(out, "BODY_PR") {
		t.Fatalf("dry-run listed an issue it should skip:\n%s", out)
	}
	got := decodePrintedRequest(t, out)
	if _, ok := mapWithoutModel(t, out); !ok {
		t.Fatal("model key present")
	}
	assertRequest(t, got, 15, "older", "BODY_OLDER", "develop")

	t.Run("includes model and does not need a key", func(t *testing.T) {
		rest := &fakeREST{branch: "develop", pages: map[int][]byte{1: oneIssue(t, 15, "older", "BODY_OLDER")}}
		var stdout, stderr bytes.Buffer
		err := Run(context.Background(), Options{Repo: "acme/widgets", Max: 1, DryRun: true}, Deps{
			REST: rest,
			HTTP: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				t.Errorf("cursor called")
				return nil, errors.New("network disabled")
			})},
			Getenv: func(name string) string {
				if name == "GCF_BUILDER_MODEL" {
					return "composer-2"
				}
				return ""
			},
		}, &stdout, &stderr)
		if err != nil {
			t.Fatal(err)
		}
		got := decodePrintedRequest(t, stdout.String())
		if got.Model == nil || got.Model.ID != "composer-2" {
			t.Fatalf("model %#v\n%s", got.Model, stdout.String())
		}
		for _, call := range rest.snapshot() {
			if call.Method != http.MethodGet {
				t.Fatalf("write: %s %s", call.Method, call.Path)
			}
		}
	})
}

func TestRunRateLimitStops(t *testing.T) {
	t.Run("cursor", func(t *testing.T) {
		res := runAgents(t, twoIssues(t), Options{Repo: "acme/widgets", Max: 2}, envWith(testKey, ""), "main", func(w http.ResponseWriter, r *http.Request, body []byte, n int) {
			if n != 1 {
				t.Errorf("started a later issue after HTTP 429")
				http.Error(w, "continued", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(w, `{"error":{"code":"rate_limit_exceeded","message":"too many agents"}}`)
		})
		if res.err == nil || !strings.Contains(res.err.Error(), "stopped:") || !strings.Contains(res.err.Error(), "HTTP 429") || !strings.Contains(res.err.Error(), "too many agents") || !strings.Contains(res.err.Error(), "no further issues were started") {
			t.Fatalf("err: %v", res.err)
		}
		want := []string{
			"GET repo",
			"GET issues",
			"POST labels #1 in-progress",
			"POST cursor",
			"POST labels #1 needs-attention",
			"POST comment #1",
		}
		if got := strings.Join(res.rec.events(), "\n"); got != strings.Join(want, "\n") {
			t.Fatalf("events:\n%s", got)
		}
		comments := res.rest.comments()
		if len(comments) != 1 || !strings.Contains(comments[0], "HTTP 429") || !strings.Contains(comments[0], "too many agents") {
			t.Fatalf("comments: %#v", comments)
		}
	})

	t.Run("github list", func(t *testing.T) {
		var hits atomic.Int32
		rest := &fakeREST{
			branch: "main",
			pages:  map[int][]byte{1: twoIssues(t)},
			fail: func(method, path string) error {
				if method == http.MethodGet && strings.Contains(path, "/issues?") {
					return &api.HTTPError{StatusCode: http.StatusTooManyRequests, Message: "secondary rate limit"}
				}
				return nil
			},
		}
		var stdout, stderr bytes.Buffer
		err := Run(context.Background(), Options{Repo: "acme/widgets", Max: 2}, Deps{
			REST: rest,
			HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				hits.Add(1)
				return nil, errors.New("network disabled")
			})},
			Getenv: func(name string) string {
				if name == "CURSOR_API_KEY" {
					return testKey
				}
				return ""
			},
		}, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), "stopped:") || !strings.Contains(err.Error(), "GitHub") || !strings.Contains(err.Error(), "HTTP 429") || !strings.Contains(err.Error(), "secondary rate limit") {
			t.Fatalf("err: %v", err)
		}
		if hits.Load() != 0 {
			t.Fatal("cursor was called")
		}
		for _, call := range rest.snapshot() {
			if call.Method != http.MethodGet {
				t.Fatalf("write after GitHub 429: %s %s", call.Method, call.Path)
			}
		}
		if strings.Contains(stdout.String(), "in-progress") {
			t.Fatalf("stdout:\n%s", stdout.String())
		}
	})
}

func TestListContinuesPastIneligiblePage(t *testing.T) {
	page1 := make([]ghIssue, pageSize)
	for i := 0; i < pageSize; i++ {
		page1[i] = ghIssue{
			Number:    1000 + i,
			Title:     "page-one-busy",
			Body:      strPtr("BODY_BUSY_PAGE"),
			State:     "open",
			CreatedAt: time.Date(2020, 1, 1, 0, 0, i, 0, time.UTC),
			Labels:    []ghLabel{{Name: labelTodo}, {Name: labelInProgress}},
		}
	}
	page2 := []ghIssue{{
		Number:    5,
		Title:     "page-two-ready",
		Body:      strPtr("BODY_PAGE_TWO"),
		State:     "open",
		CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Labels:    []ghLabel{{Name: labelTodo}},
	}}
	rest := &fakeREST{
		branch: "main",
		pages: map[int][]byte{
			1: mustJSON(t, page1),
			2: mustJSON(t, page2),
		},
	}
	rec := &recorder{}
	rest.events = rec
	srv := cursorServer(t, rec, nil)
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), Options{Repo: "acme/widgets", Max: 1}, Deps{
		REST:     rest,
		Endpoint: srv.URL + "/v1/agents",
		Getenv:   func(name string) string { return envWith(testKey, "")[name] },
	}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.bodies) != 1 || !bytes.Contains(rec.bodies[0], []byte("BODY_PAGE_TWO")) || bytes.Contains(rec.bodies[0], []byte("BODY_BUSY_PAGE")) {
		t.Fatalf("bodies: %d", len(rec.bodies))
	}
	queries := rest.issueQueries()
	if len(queries) != 2 || !strings.Contains(queries[0], "page=1") || !strings.Contains(queries[1], "page=2") || !strings.Contains(queries[0], "per_page=100") {
		t.Fatalf("queries: %v", queries)
	}
}

func TestRunNoReadyIssues(t *testing.T) {
	raw := []byte(`[{"number":2,"title":"busy","body":"x","state":"open","created_at":"2026-01-01T00:00:00Z","labels":[{"name":"todo"},{"name":"in-progress"}]}]`)
	rest := &fakeREST{branch: "main", pages: map[int][]byte{1: raw}}
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), Options{Repo: "acme/widgets", Max: 1}, Deps{
		REST: rest,
		HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Error("cursor called")
			return nil, errors.New("network disabled")
		})},
		Getenv: func(name string) string {
			if name == "CURSOR_API_KEY" {
				return testKey
			}
			return ""
		},
	}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "repo: acme/widgets\ndefault branch: main\nno issues ready to build\n" {
		t.Fatalf("stdout:\n%s", stdout.String())
	}
	for _, call := range rest.snapshot() {
		if call.Method != http.MethodGet {
			t.Fatalf("write with nothing to build: %s %s", call.Method, call.Path)
		}
	}
}

func TestRunUsesCurrentRepository(t *testing.T) {
	rest := &fakeREST{branch: "trunk", pages: map[int][]byte{1: oneIssue(t, 9, "Hosted", "BODY_HOST")}}
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), Options{Max: 1, DryRun: true}, Deps{
		REST: rest,
		HTTP: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Error("cursor called")
			return nil, errors.New("network disabled")
		})},
		Current: func() (Repository, error) {
			return Repository{Host: "ghe.example.com", Owner: "acme", Name: "widgets"}, nil
		},
		Getenv: func(string) string { return "" },
	}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	got := decodePrintedRequest(t, stdout.String())
	if len(got.Repos) != 1 || got.Repos[0].URL != "https://ghe.example.com/acme/widgets" || got.Repos[0].StartingRef != "trunk" {
		t.Fatalf("repos %+v\n%s", got.Repos, stdout.String())
	}
	if !strings.Contains(stdout.String(), "repo: acme/widgets") {
		t.Fatalf("stdout:\n%s", stdout.String())
	}
}

func TestRunRejectsBadInput(t *testing.T) {
	err := Run(context.Background(), Options{Repo: "not a repo", Max: 1, DryRun: true}, Deps{}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "OWNER/REPO") {
		t.Fatalf("repo: %v", err)
	}
	err = Run(context.Background(), Options{Repo: "acme/widgets", Max: 0, DryRun: true}, Deps{}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "max must be at least 1") {
		t.Fatalf("max: %v", err)
	}
}

func TestNullIssueBody(t *testing.T) {
	raw := []byte(`[{"number":21,"title":"Null body","body":null,"state":"open","created_at":"2026-01-01T00:00:00Z","labels":[{"name":"todo"}]}]`)
	rest := &fakeREST{branch: "main", pages: map[int][]byte{1: raw}}
	var stdout bytes.Buffer
	err := Run(context.Background(), Options{Repo: "acme/widgets", Max: 1, DryRun: true}, Deps{
		REST:   rest,
		Getenv: func(string) string { return "" },
	}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	got := decodePrintedRequest(t, stdout.String())
	if got.Prompt.Text != promptText(Issue{Number: 21, Title: "Null body"}) {
		t.Fatalf("prompt:\n%s", got.Prompt.Text)
	}
}

func assertRequest(t *testing.T, got agentRequest, number int, title, body, branch string) {
	t.Helper()
	if !got.AutoCreatePR {
		t.Fatal("autoCreatePR is false")
	}
	if got.Name != fmt.Sprintf("gcf build #%d: %s", number, title) {
		t.Fatalf("name %q", got.Name)
	}
	if len(got.Repos) != 1 || got.Repos[0].URL != "https://github.com/acme/widgets" || got.Repos[0].StartingRef != branch {
		t.Fatalf("repos %+v", got.Repos)
	}
	assertPrompt(t, got.Prompt.Text, number, title, body)
}

func assertPrompt(t *testing.T, text string, number int, title, body string) {
	t.Helper()
	for _, want := range []string{
		fmt.Sprintf("Implement GitHub issue #%d", number),
		"Title: " + title,
		body,
		"Implement only what the issue asks.",
		"Keep the change small and follow the repository's conventions.",
		"Run the repository's tests or checks if any exist.",
		"Never edit `.factory/` or `.github/workflows/`.",
		fmt.Sprintf("`Closes #%d`", number),
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("prompt missing %q\n%s", want, text)
		}
	}
}

func decodeRequest(t *testing.T, body []byte) agentRequest {
	t.Helper()
	var got agentRequest
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode request: %v\n%s", err, body)
	}
	return got
}

func decodePrintedRequest(t *testing.T, out string) agentRequest {
	t.Helper()
	idx := strings.Index(out, "{")
	if idx < 0 {
		t.Fatalf("no JSON in output:\n%s", out)
	}
	var got agentRequest
	if err := json.NewDecoder(strings.NewReader(out[idx:])).Decode(&got); err != nil {
		t.Fatalf("decode printed request: %v\n%s", err, out)
	}
	return got
}

func mapWithoutModel(t *testing.T, out string) (map[string]json.RawMessage, bool) {
	t.Helper()
	idx := strings.Index(out, "{")
	if idx < 0 {
		t.Fatal(out)
	}
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(strings.NewReader(out[idx:])).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	_, ok := raw["model"]
	return raw, !ok
}

func envWith(key, model string) map[string]string {
	return map[string]string{
		"CURSOR_API_KEY":    key,
		"GCF_BUILDER_MODEL": model,
	}
}

type runResult struct {
	stdout string
	stderr string
	err    error
	rest   *fakeREST
	rec    *recorder
}

func runAgents(t *testing.T, issues []byte, opts Options, env map[string]string, branch string, handle cursorHandler) runResult {
	t.Helper()
	rec := &recorder{}
	rest := &fakeREST{events: rec, branch: branch, pages: map[int][]byte{1: issues}}
	srv := cursorServer(t, rec, handle)
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), opts, Deps{
		REST:     rest,
		Endpoint: srv.URL + "/v1/agents",
		Getenv: func(name string) string {
			return env[name]
		},
	}, &stdout, &stderr)
	return runResult{stdout: stdout.String(), stderr: stderr.String(), err: err, rest: rest, rec: rec}
}

type cursorHandler func(w http.ResponseWriter, r *http.Request, body []byte, n int)

func cursorServer(t *testing.T, rec *recorder, handle cursorHandler) *httptest.Server {
	t.Helper()
	if handle == nil {
		handle = func(w http.ResponseWriter, r *http.Request, body []byte, n int) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"agent":{"url":"https://cursor.com/agents/bc-%d","id":"bc-%d"},"run":{"id":"run-%d"}}`, n, n, n)
		}
	}
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read cursor body: %v", err)
			http.Error(w, "read", http.StatusInternalServerError)
			return
		}
		rec.addCursor(r, body)
		if r.URL.Path != "/v1/agents" || r.Method != http.MethodPost {
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		handle(w, r, body, int(n.Add(1)))
	}))
	t.Cleanup(srv.Close)
	return srv
}

type recorder struct {
	mu     sync.Mutex
	log    []string
	bodies [][]byte
	auths  []string
	types  []string
}

func (r *recorder) add(event string) {
	r.mu.Lock()
	r.log = append(r.log, event)
	r.mu.Unlock()
}

func (r *recorder) addCursor(req *http.Request, body []byte) {
	r.mu.Lock()
	r.log = append(r.log, "POST cursor")
	r.bodies = append(r.bodies, append([]byte(nil), body...))
	r.auths = append(r.auths, req.Header.Get("Authorization"))
	r.types = append(r.types, req.Header.Get("Content-Type"))
	r.mu.Unlock()
}

func (r *recorder) events() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.log))
	copy(out, r.log)
	return out
}

type call struct {
	Method string
	Path   string
	Body   string
}

type fakeREST struct {
	mu     sync.Mutex
	calls  []call
	events *recorder
	branch string
	pages  map[int][]byte
	fail   func(method, path string) error
}

func (f *fakeREST) RequestWithContext(_ context.Context, method, path string, body io.Reader) (*http.Response, error) {
	var raw []byte
	if body != nil {
		var err error
		raw, err = io.ReadAll(body)
		if err != nil {
			return nil, err
		}
	}
	f.mu.Lock()
	f.calls = append(f.calls, call{Method: method, Path: path, Body: string(raw)})
	f.mu.Unlock()
	if f.events != nil {
		f.events.add(eventName(method, path, raw))
	}
	if f.fail != nil {
		if err := f.fail(method, path); err != nil {
			return nil, err
		}
	}
	switch {
	case method == http.MethodGet && strings.Contains(path, "/issues?"):
		payload, ok := f.pages[pageNumber(path)]
		if !ok {
			payload = []byte("[]")
		}
		return jsonResponse(payload), nil
	case method == http.MethodGet:
		branch := f.branch
		if branch == "" {
			branch = "main"
		}
		return jsonResponse([]byte(`{"default_branch":"` + branch + `"}`)), nil
	case method == http.MethodPost:
		return jsonResponse([]byte(`{}`)), nil
	default:
		return nil, fmt.Errorf("unexpected %s %s", method, path)
	}
}

func (f *fakeREST) snapshot() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]call, len(f.calls))
	copy(out, f.calls)
	return out
}

func (f *fakeREST) issueQueries() []string {
	var out []string
	for _, call := range f.snapshot() {
		if call.Method == http.MethodGet && strings.Contains(call.Path, "/issues?") {
			out = append(out, call.Path)
		}
	}
	return out
}

func (f *fakeREST) comments() []string {
	var out []string
	for _, call := range f.snapshot() {
		if !strings.Contains(call.Path, "/comments") {
			continue
		}
		var payload commentPayload
		if err := json.Unmarshal([]byte(call.Body), &payload); err != nil {
			out = append(out, call.Body)
			continue
		}
		out = append(out, payload.Body)
	}
	return out
}

func eventName(method, path string, body []byte) string {
	switch {
	case method == http.MethodGet && strings.Contains(path, "/issues?"):
		return "GET issues"
	case method == http.MethodGet:
		return "GET repo"
	case method == http.MethodPost && strings.Contains(path, "/labels"):
		var payload labelsPayload
		_ = json.Unmarshal(body, &payload)
		return fmt.Sprintf("POST labels #%s %s", issueNumber(path), strings.Join(payload.Labels, ","))
	case method == http.MethodPost && strings.Contains(path, "/comments"):
		return "POST comment #" + issueNumber(path)
	default:
		return method + " " + path
	}
}

func issueNumber(path string) string {
	const marker = "/issues/"
	i := strings.Index(path, marker)
	if i < 0 {
		return ""
	}
	rest := path[i+len(marker):]
	number, _, _ := strings.Cut(rest, "/")
	return number
}

func pageNumber(path string) int {
	parsed, err := url.Parse(path)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(parsed.Query().Get("page"))
	return n
}

func jsonResponse(body []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type ghIssue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        *string   `json:"body"`
	State       string    `json:"state"`
	CreatedAt   time.Time `json:"created_at"`
	Labels      []ghLabel `json:"labels"`
	PullRequest any       `json:"pull_request,omitempty"`
}

type ghLabel struct {
	Name string `json:"name"`
}

func strPtr(s string) *string { return &s }

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func mixedIssues(t *testing.T) []byte {
	t.Helper()
	earliest := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	older := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	newest := time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)
	return mustJSON(t, []ghIssue{
		{Number: 16, Title: "newer", Body: strPtr("BODY_NEWER"), State: "open", CreatedAt: newer, Labels: []ghLabel{{Name: labelTodo}}},
		{Number: 4, Title: "pr", Body: strPtr("BODY_PR"), State: "open", CreatedAt: earliest, Labels: []ghLabel{{Name: labelTodo}}, PullRequest: map[string]string{"url": "https://github.com/acme/widgets/pull/4"}},
		{Number: 2, Title: "busy", Body: strPtr("BODY_BUSY"), State: "open", CreatedAt: earliest, Labels: []ghLabel{{Name: labelTodo}, {Name: labelInProgress}}},
		{Number: 6, Title: "attn", Body: strPtr("BODY_ATTN"), State: "open", CreatedAt: earliest, Labels: []ghLabel{{Name: labelTodo}, {Name: labelNeedsAttention}}},
		{Number: 7, Title: "closed", Body: strPtr("BODY_CLOSED"), State: "closed", CreatedAt: earliest, Labels: []ghLabel{{Name: labelTodo}}},
		{Number: 8, Title: "other", Body: strPtr("BODY_OTHER"), State: "open", CreatedAt: earliest, Labels: []ghLabel{{Name: "bug"}}},
		{Number: 15, Title: "older", Body: strPtr("BODY_OLDER"), State: "open", CreatedAt: older, Labels: []ghLabel{{Name: "bug"}, {Name: labelTodo}}},
		{Number: 17, Title: "newest", Body: strPtr("BODY_NEWEST"), State: "open", CreatedAt: newest, Labels: []ghLabel{{Name: labelTodo}}},
	})
}

func twoIssues(t *testing.T) []byte {
	t.Helper()
	return mustJSON(t, []ghIssue{
		{Number: 2, Title: "second", Body: strPtr("BODY_SECOND"), State: "open", CreatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Labels: []ghLabel{{Name: labelTodo}}},
		{Number: 1, Title: "first", Body: strPtr("BODY_FIRST"), State: "open", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Labels: []ghLabel{{Name: labelTodo}}},
	})
}

func oneIssue(t *testing.T, number int, title, body string) []byte {
	t.Helper()
	return mustJSON(t, []ghIssue{{
		Number:    number,
		Title:     title,
		Body:      strPtr(body),
		State:     "open",
		CreatedAt: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		Labels:    []ghLabel{{Name: labelTodo}},
	}})
}

func numbers(issues []Issue) []int {
	out := make([]int, len(issues))
	for i, issue := range issues {
		out[i] = issue.Number
	}
	return out
}

func ints(vals []int) []string {
	out := make([]string, len(vals))
	for i, n := range vals {
		out[i] = strconv.Itoa(n)
	}
	return out
}

func countRunes(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
