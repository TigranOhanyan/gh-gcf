package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/TigranOhanyan/gh-gcf/internal/build"
	"github.com/TigranOhanyan/gh-gcf/internal/pull"
	"github.com/TigranOhanyan/gh-gcf/internal/source"
)

func TestParsePullArgs(t *testing.T) {
	ref, repo, force, help, err := parsePullArgs([]string{"--force", "--source", "fork/factory", "v1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	if ref != "v1.2.3" || repo != "fork/factory" || !force || help {
		t.Fatalf("got ref %q repo %q force %v help %v", ref, repo, force, help)
	}

	ref, repo, force, help, err = parsePullArgs([]string{"main", "--source=other/repo"})
	if err != nil {
		t.Fatal(err)
	}
	if ref != "main" || repo != "other/repo" || force {
		t.Fatalf("flags after ref: ref %q repo %q force %v", ref, repo, force)
	}

	if _, _, _, _, err := parsePullArgs(nil); err == nil {
		t.Fatal("missing ref succeeded")
	}
	if _, _, _, _, err := parsePullArgs([]string{"--source"}); err == nil {
		t.Fatal("source without value succeeded")
	}
	if _, _, _, _, err := parsePullArgs([]string{"--nope", "main"}); err == nil {
		t.Fatal("unknown flag succeeded")
	}
	_, _, _, help, err = parsePullArgs([]string{"--help"})
	if err != nil || !help {
		t.Fatalf("help: %v %v", help, err)
	}
}

func TestRunHelpAndUsage(t *testing.T) {
	var out, errb bytes.Buffer
	fetch := &failFetcher{}
	if err := Run([]string{"--help"}, t.TempDir(), &out, &errb, fetch); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "gh gcf pull <ref>") || !strings.Contains(out.String(), "gh gcf build") || fetch.called {
		t.Fatalf("help out %q called %v", out.String(), fetch.called)
	}
	out.Reset()
	if err := Run(nil, t.TempDir(), &out, &errb, fetch); err == nil || !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("missing command: %v", err)
	}
	if err := Run([]string{"init"}, t.TempDir(), &out, &errb, fetch); err == nil || fetch.called {
		t.Fatal("unknown command should fail before fetch")
	}
}

func TestRunRequiresRepoRootBeforeFetch(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	fetch := &failFetcher{}
	err := Run([]string{"pull", "main"}, sub, &bytes.Buffer{}, &bytes.Buffer{}, fetch)
	if err == nil || !strings.Contains(err.Error(), "root of a host git repository") {
		t.Fatalf("error: %v", err)
	}
	if fetch.called {
		t.Fatal("fetcher was called")
	}

	var out bytes.Buffer
	fetch.archive = pullArchive(t, map[string]string{"labels.tf": "v1"})
	fetch.commit = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := Run([]string{"pull", "main"}, root, &out, &bytes.Buffer{}, fetch); err != nil {
		t.Fatal(err)
	}
	if fetch.repo != source.DefaultRepo || fetch.ref != "main" {
		t.Fatalf("fetch repo %q ref %q", fetch.repo, fetch.ref)
	}
	if !strings.Contains(out.String(), "1 added, 0 updated, 0 removed, 0 skipped") {
		t.Fatalf("stdout:\n%s", out.String())
	}
}

func TestRunGitFileAndFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /somewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fetch := &failFetcher{
		archive: pullArchive(t, map[string]string{"labels.tf": "v1"}),
		commit:  "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	var out, errb bytes.Buffer
	if err := Run([]string{"pull", "--source", "fork/factory", "latest", "--force"}, dir, &out, &errb, fetch); err != nil {
		t.Fatal(err)
	}
	if fetch.repo != "fork/factory" || fetch.ref != "latest" {
		t.Fatalf("fetch repo %q ref %q", fetch.repo, fetch.ref)
	}
	lock, err := os.ReadFile(filepath.Join(dir, ".factory", "gcf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(lock)
	if !strings.Contains(text, `"source": "fork/factory"`) || !strings.Contains(text, `"ref": "latest"`) || !strings.Contains(text, fetch.commit) {
		t.Fatalf("lock:\n%s", text)
	}
}

func TestRunLatestPrintsReleaseAndWarns(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	fetch := &failFetcher{
		archive:    pullArchive(t, map[string]string{"labels.tf": "v1", "github/workflows/factory.yml": "wf"}),
		commit:     "cccccccccccccccccccccccccccccccccccccccc",
		releaseTag: "v9.9.9",
	}
	var out, errb bytes.Buffer
	if err := Run([]string{"pull", "latest"}, dir, &out, &errb, fetch); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".factory", "labels.tf"), []byte("edited"), 0o644); err != nil {
		t.Fatal(err)
	}
	fetch.archive = pullArchive(t, map[string]string{"labels.tf": "v2", "github/workflows/factory.yml": "wf"})
	fetch.commit = "dddddddddddddddddddddddddddddddddddddddd"
	out.Reset()
	errb.Reset()
	if err := Run([]string{"pull", "latest"}, dir, &out, &errb, fetch); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errb.String(), "warning: .factory/labels.tf has local changes") {
		t.Fatalf("stderr:\n%s", errb.String())
	}
	if strings.Contains(out.String(), "warning:") {
		t.Fatalf("stdout contains warning:\n%s", out.String())
	}
	stdout := out.String()
	if !strings.Contains(stdout, "release: v9.9.9") || !strings.Contains(stdout, "ref: latest") || !strings.Contains(stdout, "skipped:\n  .factory/labels.tf") {
		t.Fatalf("stdout:\n%s", stdout)
	}
	if !strings.Contains(stdout, "0 added, 0 updated, 0 removed, 1 skipped") {
		t.Fatalf("stdout:\n%s", stdout)
	}
	body, err := os.ReadFile(filepath.Join(dir, ".factory", "labels.tf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "edited" {
		t.Fatalf("edited file was overwritten: %q", body)
	}
}

func TestParseBuildArgs(t *testing.T) {
	opts, help, err := parseBuildArgs([]string{"--repo", "acme/widgets", "--max", "3", "--dry-run"})
	if err != nil {
		t.Fatal(err)
	}
	if help || opts.Repo != "acme/widgets" || opts.Max != 3 || !opts.DryRun {
		t.Fatalf("opts %+v help %v", opts, help)
	}

	opts, help, err = parseBuildArgs([]string{"--max=2", "--repo=acme/widgets", "--dry-run=false"})
	if err != nil {
		t.Fatal(err)
	}
	if help || opts.Repo != "acme/widgets" || opts.Max != 2 || opts.DryRun {
		t.Fatalf("flags after form: %+v help %v", opts, help)
	}

	opts, _, err = parseBuildArgs(nil)
	if err != nil || opts.Max != 1 || opts.Repo != "" || opts.DryRun {
		t.Fatalf("defaults: %+v %v", opts, err)
	}
	for _, args := range [][]string{
		{"--max"},
		{"--max", "0"},
		{"--max", "-1"},
		{"--max", "nope"},
		{"--repo"},
		{"--bogus"},
		{"extra"},
	} {
		if _, _, err := parseBuildArgs(args); err == nil {
			t.Fatalf("parse %q succeeded", args)
		}
	}
	_, help, err = parseBuildArgs([]string{"--help"})
	if err != nil || !help {
		t.Fatalf("help: %v %v", help, err)
	}
}

func TestRunBuildHelp(t *testing.T) {
	fetch := &failFetcher{}
	var out, errb bytes.Buffer
	if err := Run([]string{"build", "--help"}, t.TempDir(), &out, &errb, fetch); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "gh gcf build") || !strings.Contains(out.String(), "--dry-run") || fetch.called {
		t.Fatalf("out %q called %v", out.String(), fetch.called)
	}
	if err := Run([]string{"build", "--bogus"}, t.TempDir(), &out, &errb, fetch); err == nil || fetch.called {
		t.Fatalf("unknown flag: %v called %v", err, fetch.called)
	}
}

func TestRunBuildDryRunUsesFlags(t *testing.T) {
	issues := []byte(`[
		{"number":2,"title":"beta-unique","body":"later","state":"open","created_at":"2026-02-01T00:00:00Z","labels":[{"name":"todo"}]},
		{"number":1,"title":"alpha-unique","body":"earlier","state":"open","created_at":"2026-01-01T00:00:00Z","labels":[{"name":"todo"}]}
	]`)
	gh := &scriptREST{issues: issues}
	var out, errb bytes.Buffer
	err := runBuild([]string{"--repo", "acme/widgets", "--max", "1", "--dry-run"}, &out, &errb, &build.Deps{
		REST: gh,
		Getenv: func(string) string {
			return ""
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "repo: acme/widgets") || !strings.Contains(text, "alpha-unique") || !strings.Contains(text, "earlier") {
		t.Fatalf("stdout:\n%s", text)
	}
	if strings.Contains(text, "beta-unique") {
		t.Fatalf("max was not applied:\n%s", text)
	}
	if !strings.Contains(text, "POST https://api.cursor.com/v1/agents") || !strings.Contains(text, `"autoCreatePR": true`) {
		t.Fatalf("request:\n%s", text)
	}
	for _, call := range gh.calls {
		if !strings.HasPrefix(call, "GET ") {
			t.Fatalf("dry-run write: %s", call)
		}
	}

	gh.calls = nil
	out.Reset()
	err = runBuild([]string{"--repo", "acme/widgets"}, &out, &errb, &build.Deps{
		REST: gh,
		Getenv: func(string) string {
			return " \n"
		},
	})
	if err == nil || !strings.Contains(err.Error(), "CURSOR_API_KEY is not set") {
		t.Fatalf("missing key: %v", err)
	}
	if len(gh.calls) != 0 {
		t.Fatalf("missing key called GitHub: %v", gh.calls)
	}
}

type scriptREST struct {
	issues []byte
	calls  []string
}

func (s *scriptREST) RequestWithContext(_ context.Context, method, path string, body io.Reader) (*http.Response, error) {
	s.calls = append(s.calls, method+" "+path)
	if body != nil {
		if _, err := io.Copy(io.Discard, body); err != nil {
			return nil, err
		}
	}
	if method != http.MethodGet {
		return nil, errors.New("unexpected write " + method + " " + path)
	}
	payload := []byte(`{"default_branch":"main"}`)
	if strings.Contains(path, "/issues?") {
		payload = s.issues
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(payload)),
		Header:     make(http.Header),
	}, nil
}

func TestFormatSummaryOmitsEmptyRelease(t *testing.T) {
	text := FormatSummary("o/r", "main", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "", pull.Summary{})
	if strings.Contains(text, "release:") {
		t.Fatalf("release line:\n%s", text)
	}
	if !strings.Contains(text, "0 added, 0 updated, 0 removed, 0 skipped\n") {
		t.Fatalf("summary:\n%s", text)
	}
}

type failFetcher struct {
	archive    []byte
	commit     string
	releaseTag string
	repo       string
	ref        string
	called     bool
}

func pullArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	const prefix = "factory-abc/"
	for name, body := range files {
		hdr := &tar.Header{
			Name:     prefix + name,
			Mode:     0o644,
			Size:     int64(len(body)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (f *failFetcher) Fetch(_ context.Context, repo, ref string) (source.Fetched, error) {
	f.called = true
	f.repo = repo
	f.ref = ref
	if f.archive == nil {
		return source.Fetched{}, os.ErrNotExist
	}
	return source.Fetched{Commit: f.commit, ReleaseTag: f.releaseTag, Archive: f.archive}, nil
}
