package label_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wiremock/go-wiremock"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const owner = "octo"
const repoName = "app"
const token = "test-token"

func Test_Label_should_create_a_missing_label(t *testing.T) {
	resetWireMock(t)
	stubGet(t, "todo", http.StatusNotFound, `{"message":"Not Found"}`)
	stubPost(t, "todo", "ededed", http.StatusCreated, `{"id":1,"node_id":"x","url":"http://example","name":"todo","color":"ededed","default":false}`)
	repoDir := setupFactory(t, []configLabel{{name: "todo", color: "ededed"}})
	id := runTick(t, repoDir)
	startOfTest := time.Now().Add(-time.Second)

	err := runLabel(t, repoDir, id, token)
	require.NoError(t, err)
	startOfCheck := time.Now().Add(time.Second)

	requirePost(t, "todo", "ededed", token, 1)
	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("✅ [%s] [label] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Equal(t, "✅ [todo] created", strings.TrimSpace(string(body)))

	rows := labelsAtTip(t, repoDir)
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, "todo", row.Name)
	assert.Equal(t, "ededed", row.Color)
	assert.True(t, row.CreatedAt.Valid)
	registeredAt, err := time.Parse(time.RFC3339Nano, row.RegisteredAt)
	require.NoError(t, err)
	assert.WithinRange(t, registeredAt, startOfTest, startOfCheck)
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt.String)
	require.NoError(t, err)
	assert.WithinRange(t, createdAt, startOfTest, startOfCheck)

}

func Test_Label_should_record_a_label_that_already_exists(t *testing.T) {
	resetWireMock(t)
	stubGet(t, "todo", http.StatusOK, `{"id":1,"node_id":"x","url":"http://example","name":"todo","color":"111111","default":false}`)
	repoDir := setupFactory(t, []configLabel{{name: "todo", color: "ededed"}})
	id := runTick(t, repoDir)
	startOfTest := time.Now().Add(-time.Second)

	err := runLabel(t, repoDir, id, token)
	require.NoError(t, err)
	startOfCheck := time.Now().Add(time.Second)

	requirePost(t, "todo", "ededed", token, 0)
	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("✅ [%s] [label] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Empty(t, strings.TrimSpace(string(body)))

	rows := labelsAtTip(t, repoDir)
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, "todo", row.Name)
	assert.Equal(t, "ededed", row.Color)
	assert.False(t, row.CreatedAt.Valid)
	registeredAt, err := time.Parse(time.RFC3339Nano, row.RegisteredAt)
	require.NoError(t, err)
	assert.WithinRange(t, registeredAt, startOfTest, startOfCheck)
}

func Test_Label_should_leave_a_label_that_exists_in_both_places(t *testing.T) {
	resetWireMock(t)
	stubGet(t, "todo", http.StatusOK, `{"id":1,"node_id":"x","url":"http://example","name":"todo","color":"111111","default":false}`)
	repoDir := setupFactory(t, []configLabel{{name: "todo", color: "ededed"}})
	id := runTick(t, repoDir)
	err := runLabel(t, repoDir, id, token)
	require.NoError(t, err)
	resetWireMock(t)
	stubGet(t, "todo", http.StatusOK, `{"id":1,"node_id":"x","url":"http://example","name":"todo","color":"111111","default":false}`)

	err = runLabel(t, repoDir, id, token)
	require.NoError(t, err)

	requirePost(t, "todo", "ededed", token, 0)
	rows := labelsAtTip(t, repoDir)
	require.Len(t, rows, 1)
	assert.False(t, rows[0].CreatedAt.Valid)
}

func Test_Label_should_keep_the_row_when_create_fails(t *testing.T) {
	resetWireMock(t)
	stubGet(t, "todo", http.StatusNotFound, `{"message":"Not Found"}`)
	stubPost(t, "todo", "ededed", http.StatusUnprocessableEntity, `{"message":"Validation Failed"}`)
	repoDir := setupFactory(t, []configLabel{{name: "todo", color: "ededed"}})
	id := runTick(t, repoDir)

	err := runLabel(t, repoDir, id, token)
	require.Error(t, err)

	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("❌ [%s] [label] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	bodyText := strings.TrimSpace(string(body))
	assert.Contains(t, bodyText, "❌ [todo]")
	assert.Contains(t, bodyText, "Validation Failed")
	assert.NotContains(t, bodyText, "failed to create")

	rows := labelsAtTip(t, repoDir)
	require.Len(t, rows, 1)
	assert.Equal(t, "todo", rows[0].Name)
	assert.Equal(t, "ededed", rows[0].Color)
	assert.False(t, rows[0].CreatedAt.Valid)
}

func Test_Label_should_set_created_at_when_a_failed_label_is_created_later(t *testing.T) {
	resetWireMock(t)
	stubGet(t, "todo", http.StatusNotFound, `{"message":"Not Found"}`)
	stubPost(t, "todo", "ededed", http.StatusUnprocessableEntity, `{"message":"Validation Failed"}`)
	repoDir := setupFactory(t, []configLabel{{name: "todo", color: "ededed"}})
	id := runTick(t, repoDir)
	err := runLabel(t, repoDir, id, token)
	require.Error(t, err)
	resetWireMock(t)
	stubGet(t, "todo", http.StatusNotFound, `{"message":"Not Found"}`)
	stubPost(t, "todo", "ededed", http.StatusCreated, `{"id":1,"node_id":"x","url":"http://example","name":"todo","color":"ededed","default":false}`)
	startOfTest := time.Now().Add(-time.Second)

	err = runLabel(t, repoDir, id, token)
	require.NoError(t, err)
	startOfCheck := time.Now().Add(time.Second)

	rows := labelsAtTip(t, repoDir)
	require.Len(t, rows, 1)
	assert.Equal(t, "ededed", rows[0].Color)
	assert.True(t, rows[0].CreatedAt.Valid)
	createdAt, err := time.Parse(time.RFC3339Nano, rows[0].CreatedAt.String)
	require.NoError(t, err)
	assert.WithinRange(t, createdAt, startOfTest, startOfCheck)
}

func Test_Label_should_stop_after_the_first_create_error(t *testing.T) {
	resetWireMock(t)
	stubGet(t, "alpha", http.StatusNotFound, `{"message":"Not Found"}`)
	stubPost(t, "alpha", "aaaaaa", http.StatusCreated, `{"id":1,"node_id":"x","url":"http://example","name":"alpha","color":"aaaaaa","default":false}`)
	stubGet(t, "beta", http.StatusNotFound, `{"message":"Not Found"}`)
	stubPost(t, "beta", "bbbbbb", http.StatusInternalServerError, `{"message":"server failed"}`)
	repoDir := setupFactory(t, []configLabel{
		{name: "alpha", color: "aaaaaa"},
		{name: "beta", color: "bbbbbb"},
		{name: "gamma", color: "cccccc"},
	})
	id := runTick(t, repoDir)

	err := runLabel(t, repoDir, id, token)
	require.Error(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	bodyText := strings.TrimSpace(string(body))
	assert.Contains(t, bodyText, "✅ [alpha] created")
	assert.Contains(t, bodyText, "❌ [beta]")
	assert.Contains(t, bodyText, "server failed")
	requireGetCount(t, "gamma", 0)
	rows := labelsAtTip(t, repoDir)
	require.Len(t, rows, 2)
	assert.Equal(t, "alpha", rows[0].Name)
	assert.True(t, rows[0].CreatedAt.Valid)
	assert.Equal(t, "beta", rows[1].Name)
	assert.False(t, rows[1].CreatedAt.Valid)
}

func Test_Label_should_stop_when_the_label_lookup_fails(t *testing.T) {
	resetWireMock(t)
	stubGet(t, "todo", http.StatusUnauthorized, `{"message":"Bad credentials"}`)
	repoDir := setupFactory(t, []configLabel{{name: "todo", color: "ededed"}})
	id := runTick(t, repoDir)

	err := runLabel(t, repoDir, id, token)
	require.Error(t, err)

	rows := labelsAtTip(t, repoDir)
	assert.Empty(t, rows)
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "❌ [todo]")
	assert.Contains(t, string(body), "Bad credentials")
}

func Test_Label_should_commit_when_config_has_no_labels(t *testing.T) {
	resetWireMock(t)
	repoDir := setupFactory(t, nil)
	id := runTick(t, repoDir)

	err := runLabel(t, repoDir, id, token)
	require.NoError(t, err)

	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("✅ [%s] [label] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Empty(t, strings.TrimSpace(string(body)))
	requests, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	assert.Empty(t, requests.Requests)
}

func Test_Label_should_use_gh_auth_token_when_the_environment_has_none(t *testing.T) {
	resetWireMock(t)
	stubGet(t, "todo", http.StatusOK, `{"id":1,"node_id":"x","url":"http://example","name":"todo","color":"ededed","default":false}`)
	repoDir := setupFactory(t, []configLabel{{name: "todo", color: "ededed"}})
	id := runTick(t, repoDir)
	script := writeTokenScript(t)
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_PATH", script)

	err := runLabel(t, repoDir, id, "")
	require.NoError(t, err)

	requireGetToken(t, "todo", "script-token")
}

func Test_Label_should_create_a_label_with_an_installation_token(t *testing.T) {
	resetWireMock(t)
	stubInstallation(t, 42)
	stubInstallationToken(t, 42, http.StatusCreated, `{"token":"ghs_test"}`)
	stubGet(t, "todo", http.StatusNotFound, `{"message":"Not Found"}`)
	stubPost(t, "todo", "ededed", http.StatusCreated, `{"id":1,"node_id":"x","url":"http://example","name":"todo","color":"ededed","default":false}`)
	repoDir := setupFactory(t, []configLabel{{name: "todo", color: "ededed"}})
	id := runTick(t, repoDir)
	pemText := testAppKey(t)
	t.Setenv("FACTORY_GITHUB_APP_ID", "123")
	t.Setenv("FACTORY_GITHUB_APP_PRIVATE_KEY", pemText)
	t.Setenv("GH_TOKEN", "")
	startOfTest := time.Now().Add(-time.Second)

	err := runLabel(t, repoDir, id, "")
	require.NoError(t, err)
	startOfCheck := time.Now().Add(time.Second)

	requireRequestOrder(t, []string{
		"GET /repos/octo/app/installation",
		"POST /app/installations/42/access_tokens",
		"GET /repos/octo/app/labels/todo",
		"POST /repos/octo/app/labels",
	})
	requireJWT(t, "GET", "/repos/octo/app/installation")
	requireGetToken(t, "todo", "ghs_test")
	requirePost(t, "todo", "ededed", "ghs_test", 1)

	rows := labelsAtTip(t, repoDir)
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, "todo", row.Name)
	assert.Equal(t, "ededed", row.Color)
	assert.True(t, row.CreatedAt.Valid)
	registeredAt, err := time.Parse(time.RFC3339Nano, row.RegisteredAt)
	require.NoError(t, err)
	assert.WithinRange(t, registeredAt, startOfTest, startOfCheck)
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt.String)
	require.NoError(t, err)
	assert.WithinRange(t, createdAt, startOfTest, startOfCheck)
}

func Test_Label_should_stop_when_the_token_exchange_fails(t *testing.T) {
	resetWireMock(t)
	stubInstallation(t, 42)
	stubInstallationToken(t, 42, http.StatusUnauthorized, `{"message":"Bad credentials"}`)
	repoDir := setupFactory(t, []configLabel{{name: "todo", color: "ededed"}})
	id := runTick(t, repoDir)
	pemText := testAppKey(t)
	t.Setenv("FACTORY_GITHUB_APP_ID", "123")
	t.Setenv("FACTORY_GITHUB_APP_PRIVATE_KEY", pemText)
	t.Setenv("GH_TOKEN", "")

	err := runLabel(t, repoDir, id, "")
	require.Error(t, err)

	requireGetCount(t, "todo", 0)
	rows := labelsAtTip(t, repoDir)
	assert.Empty(t, rows)
	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("❌ [%s] [label] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "Bad credentials")
}

func Test_Label_should_refuse_an_app_id_without_a_private_key(t *testing.T) {
	repoDir := setupFactory(t, []configLabel{{name: "todo", color: "ededed"}})
	id := runTick(t, repoDir)
	t.Setenv("FACTORY_GITHUB_APP_ID", "123")
	t.Setenv("FACTORY_GITHUB_APP_PRIVATE_KEY", "")

	err := runLabel(t, repoDir, id, token)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "github app credentials are incomplete")
	rows := labelsAtTip(t, repoDir)
	assert.Empty(t, rows)
	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("[%s] [tick] [octo/app] created", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
}

func Test_Label_should_require_a_tick_id(t *testing.T) {
	_, err := runRoot(t, "label")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tick")
}

type configLabel struct {
	name  string
	color string
}

func setupFactory(t *testing.T, labels []configLabel) (repoDir string) {
	t.Helper()
	repoDir = initFactory(t)
	mustRoot(t, "setup repo add --owner octo --name app")
	for _, label := range labels {
		line := fmt.Sprintf("setup label add --owner octo --repo app --name %q --color %q", label.name, label.color)
		mustRoot(t, line)
	}
	commitConfig(t, repoDir)
	useStateBranch(t, repoDir)
	return
}

func initFactory(t *testing.T) (repoDir string) {
	t.Helper()
	repoDir = filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)
	_ = mustGit(t, repoDir, "init -b main")
	_ = mustGit(t, repoDir, "config user.email test@example.com")
	_ = mustGit(t, repoDir, "config user.name test")
	_ = mustGit(t, repoDir, "commit --allow-empty -m init")
	t.Chdir(repoDir)
	return
}

func commitConfig(t *testing.T, repoDir string) {
	t.Helper()
	_ = mustGit(t, repoDir, "add -A")
	_ = mustGit(t, repoDir, "commit -m config")
}

func useStateBranch(t *testing.T, repoDir string) {
	t.Helper()
	_ = mustGit(t, repoDir, "checkout -b state")
	_ = mustGit(t, repoDir, "checkout main")
}

func runTick(t *testing.T, repoDir string) (id string) {
	t.Helper()
	t.Chdir(repoDir)
	stdout := mustRoot(t, "tick")
	id = strings.TrimSpace(stdout)
	require.NotEmpty(t, id)
	return
}

func runLabel(t *testing.T, repoDir string, id string, authToken string) (err error) {
	t.Helper()
	if authToken != "" {
		t.Setenv("GH_TOKEN", authToken)
	}
	t.Chdir(repoDir)
	line := fmt.Sprintf("label --tick %s", id)
	stdout, err := runRoot(t, line)
	if err == nil {
		assert.Empty(t, stdout)
	}
	return
}

func runRoot(t *testing.T, line string) (stdout string, err error) {
	t.Helper()
	command := root.NewRoot()
	var out bytes.Buffer
	var errOut bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&errOut)
	wrapped := commands.NewCmd(command)
	err = wrapped.Shell(t.Context(), line)
	stdout = out.String()
	return
}

func mustRoot(t *testing.T, line string) (stdout string) {
	t.Helper()
	stdout, err := runRoot(t, line)
	require.NoError(t, err)
	return
}

func labelsAtTip(t *testing.T, repoDir string) (rows []records.Label) {
	t.Helper()
	db := openStateTip(t, repoDir)
	defer db.Close()
	rows, err := db.Queries.ListLabels(t.Context())
	require.NoError(t, err)
	return
}

func openStateTip(t *testing.T, repoDir string) (db *statedb.DB) {
	t.Helper()
	tipPath := filepath.Join(JunkDir, t.Name()+"-state-tip.db")
	spec := "state:repos/" + owner + "/" + repoName + "/state.db"
	showLine := fmt.Sprintf("show %s", spec)
	blob := mustGit(t, repoDir, showLine)
	err := os.WriteFile(tipPath, blob, 0644)
	require.NoError(t, err)
	db, err = statedb.Open(t.Context(), tipPath)
	require.NoError(t, err)
	return
}

func resetWireMock(t *testing.T) {
	t.Helper()
	err := wiremockClient.Reset()
	require.NoError(t, err)
}

func stubInstallation(t *testing.T, id int64) {
	t.Helper()
	path := fmt.Sprintf("/repos/%s/%s/installation", owner, repoName)
	body := fmt.Sprintf(`{"id":%d}`, id)
	rule := wiremock.Get(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().
			WithStatus(int64(http.StatusOK)).
			WithHeader("Content-Type", "application/json").
			WithBody(body),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubInstallationToken(t *testing.T, id int64, status int, body string) {
	t.Helper()
	path := fmt.Sprintf("/app/installations/%d/access_tokens", id)
	rule := wiremock.Post(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().
			WithStatus(int64(status)).
			WithHeader("Content-Type", "application/json").
			WithBody(body),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubGet(t *testing.T, name string, status int, body string) {
	t.Helper()
	path := fmt.Sprintf("/repos/%s/%s/labels/%s", owner, repoName, name)
	rule := wiremock.Get(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().
			WithStatus(int64(status)).
			WithHeader("Content-Type", "application/json").
			WithBody(body),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubPost(t *testing.T, name string, color string, status int, body string) {
	t.Helper()
	path := fmt.Sprintf("/repos/%s/%s/labels", owner, repoName)
	payload := fmt.Sprintf(`{"name":%q,"color":%q}`, name, color)
	rule := wiremock.Post(wiremock.URLPathEqualTo(path)).
		WithBodyPattern(wiremock.EqualToJson(payload)).
		WillReturnResponse(
			wiremock.NewResponse().
				WithStatus(int64(status)).
				WithHeader("Content-Type", "application/json").
				WithBody(body),
		)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func requirePost(t *testing.T, name string, color string, authToken string, count int64) {
	t.Helper()
	path := fmt.Sprintf("/repos/%s/%s/labels", owner, repoName)
	payload := fmt.Sprintf(`{"name":%q,"color":%q}`, name, color)
	rule := wiremock.Post(wiremock.URLPathEqualTo(path)).
		WithHeader("Authorization", wiremock.EqualTo("Bearer "+authToken)).
		WithBodyPattern(wiremock.EqualToJson(payload))
	ok, err := wiremockClient.Verify(rule.Request(), count)
	require.NoError(t, err)
	assert.True(t, ok)
}

func requireGetCount(t *testing.T, name string, count int64) {
	t.Helper()
	path := fmt.Sprintf("/repos/%s/%s/labels/%s", owner, repoName, name)
	rule := wiremock.Get(wiremock.URLPathEqualTo(path))
	ok, err := wiremockClient.Verify(rule.Request(), count)
	require.NoError(t, err)
	assert.True(t, ok)
}

func requireGetToken(t *testing.T, name string, authToken string) {
	t.Helper()
	path := fmt.Sprintf("/repos/%s/%s/labels/%s", owner, repoName, name)
	rule := wiremock.Get(wiremock.URLPathEqualTo(path)).
		WithHeader("Authorization", wiremock.EqualTo("Bearer "+authToken))
	ok, err := wiremockClient.Verify(rule.Request(), 1)
	require.NoError(t, err)
	assert.True(t, ok)
}

func requireRequestOrder(t *testing.T, expected []string) {
	t.Helper()
	journal, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	require.Len(t, journal.Requests, len(expected))
	got := make([]string, 0, len(journal.Requests))
	last := len(journal.Requests) - 1
	for i := last; i >= 0; i-- {
		req := journal.Requests[i].Request
		line := req.Method + " " + req.URL
		got = append(got, line)
	}
	assert.Equal(t, expected, got)
}

func requireJWT(t *testing.T, method string, path string) {
	t.Helper()
	journal, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	var header string
	for _, entry := range journal.Requests {
		req := entry.Request
		match := req.Method == method && req.URL == path
		if match {
			header = req.Headers["Authorization"]
		}
	}
	require.NotEmpty(t, header)
	tokenText := strings.TrimPrefix(header, "Bearer ")
	parts := strings.Split(tokenText, ".")
	assert.Len(t, parts, 3)
}

func testAppKey(t *testing.T) (pemText string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	pemText = string(pem.EncodeToMemory(block))
	return
}

func writeTokenScript(t *testing.T) (path string) {
	t.Helper()
	path = filepath.Join(JunkDir, t.Name()+"-gh")
	script := "#!/bin/sh\nif [ \"$1\" = \"auth\" ] && [ \"$2\" = \"token\" ]; then\n  echo script-token\n  exit 0\nfi\nexit 1\n"
	err := os.WriteFile(path, []byte(script), 0755)
	require.NoError(t, err)
	return
}
