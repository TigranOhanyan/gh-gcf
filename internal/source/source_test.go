package source

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

func TestParseRepo(t *testing.T) {
	owner, name, err := ParseRepo(DefaultRepo)
	if err != nil {
		t.Fatal(err)
	}
	if owner != "TigranOhanyan" || name != "github-cursor-factory" {
		t.Fatalf("got %s/%s", owner, name)
	}

	for _, bad := range []string{
		"",
		"owner",
		"owner/",
		"/repo",
		"owner/repo/extra",
		"owner/repo ",
		"https://github.com/owner/repo",
		"owner/../repo",
		".",
		"../repo",
	} {
		if _, _, err := ParseRepo(bad); err == nil {
			t.Errorf("ParseRepo(%q) succeeded", bad)
		}
	}
}

func TestValidateRef(t *testing.T) {
	for _, ref := range []string{"latest", "v1.2.3", "main", "feature/foo", "abc1234"} {
		if err := ValidateRef(ref); err != nil {
			t.Errorf("ValidateRef(%q): %v", ref, err)
		}
	}
	for _, ref := range []string{"", ".", "..", "/main", "main/", "foo//bar", "foo/../bar", "foo/./bar", "has space", "a?b", "a#b", `a\b`} {
		if err := ValidateRef(ref); err == nil {
			t.Errorf("ValidateRef(%q) succeeded", ref)
		}
	}
}

func TestFetchLatest(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	archive := []byte{0x1f, 0x8b, 0x00}
	client := &fakeREST{bodies: map[string][]byte{
		"repos/acme/factory/releases/latest": []byte(`{"tag_name":"v1.2.3"}`),
		"repos/acme/factory/commits/v1.2.3":  []byte(`{"sha":"` + sha + `","unused":true}`),
		"repos/acme/factory/tarball/v1.2.3":  archive,
	}}
	got, err := Fetch(context.Background(), client, "acme/factory", "latest")
	if err != nil {
		t.Fatal(err)
	}
	if got.Commit != sha || got.ReleaseTag != "v1.2.3" || !bytes.Equal(got.Archive, archive) {
		t.Fatalf("got %+v", got)
	}
	wantCalls := []string{
		"GET repos/acme/factory/releases/latest",
		"GET repos/acme/factory/commits/v1.2.3",
		"GET repos/acme/factory/tarball/v1.2.3",
	}
	if strings.Join(client.calls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("calls:\n%s", strings.Join(client.calls, "\n"))
	}
}

func TestFetchLatestNoRelease(t *testing.T) {
	client := &fakeREST{
		status: map[string]int{"repos/acme/factory/releases/latest": http.StatusNotFound},
		bodies: map[string][]byte{"repos/acme/factory": []byte(`{"full_name":"acme/factory"}`)},
	}
	_, err := Fetch(context.Background(), client, "acme/factory", "latest")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "acme/factory has no releases yet") {
		t.Fatalf("error: %v", err)
	}
	if strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing release reported as missing repository: %v", err)
	}
}

func TestFetchRepositoryNotFound(t *testing.T) {
	client := &fakeREST{status: map[string]int{
		"repos/acme/factory/releases/latest": http.StatusNotFound,
		"repos/acme/factory":                 http.StatusNotFound,
	}}
	_, err := Fetch(context.Background(), client, "acme/factory", "latest")
	if err == nil || !strings.Contains(err.Error(), "repository acme/factory not found") {
		t.Fatalf("error: %v", err)
	}
	if strings.Contains(err.Error(), "no releases") {
		t.Fatalf("missing repository reported as no releases: %v", err)
	}
}

func TestFetchBranchDoesNotAskForReleases(t *testing.T) {
	const sha = "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
	client := &fakeREST{bodies: map[string][]byte{
		"repos/acme/factory/commits/main": []byte(`{"sha":"` + strings.ToUpper(sha) + `"}`),
		"repos/acme/factory/tarball/main": []byte("tar"),
	}}
	got, err := Fetch(context.Background(), client, "acme/factory", "main")
	if err != nil {
		t.Fatal(err)
	}
	if got.Commit != sha || got.ReleaseTag != "" {
		t.Fatalf("got %+v", got)
	}
	for _, call := range client.calls {
		if strings.Contains(call, "releases") {
			t.Fatalf("branch fetch requested a release: %s", call)
		}
	}
}

func TestFetchEscapesBranchWithSlash(t *testing.T) {
	const sha = "1234567890abcdef1234567890abcdef12345678"
	client := &fakeREST{bodies: map[string][]byte{
		"repos/acme/factory/commits/feature%2Ffoo": []byte(`{"sha":"` + sha + `"}`),
		"repos/acme/factory/tarball/feature%2Ffoo": []byte("tar"),
	}}
	if _, err := Fetch(context.Background(), client, "acme/factory", "feature/foo"); err != nil {
		t.Fatal(err)
	}
}

func TestFetchRefNotFound(t *testing.T) {
	client := &fakeREST{status: map[string]int{
		"repos/acme/factory/commits/missing": http.StatusNotFound,
	}}
	_, err := Fetch(context.Background(), client, "acme/factory", "missing")
	if err == nil || !strings.Contains(err.Error(), `ref "missing" not found`) {
		t.Fatalf("error: %v", err)
	}
}

func TestFetchRejectsShortResolvedSHA(t *testing.T) {
	client := &fakeREST{bodies: map[string][]byte{
		"repos/acme/factory/commits/main": []byte(`{"sha":"abc"}`),
	}}
	_, err := Fetch(context.Background(), client, "acme/factory", "main")
	if err == nil || !strings.Contains(err.Error(), "40-character") {
		t.Fatalf("error: %v", err)
	}
}

type fakeREST struct {
	bodies map[string][]byte
	status map[string]int
	calls  []string
}

func (f *fakeREST) RequestWithContext(_ context.Context, method, path string, _ io.Reader) (*http.Response, error) {
	f.calls = append(f.calls, method+" "+path)
	if code := f.status[path]; code != 0 {
		return nil, &api.HTTPError{StatusCode: code, Message: "Not Found"}
	}
	body, ok := f.bodies[path]
	if !ok {
		return nil, errors.New("unexpected path " + path)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}
