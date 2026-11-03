package label_test

import (
	"bytes"
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

	"github.com/TigranOhanyan/gh-gcf/commands/label"
	"github.com/TigranOhanyan/gh-gcf/commands/tick"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
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
	subject := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	expectedSubject := fmt.Sprintf("✅ [%s] [label] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log", "-1", "--format=%b", "state")
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

	events := eventsAtTip(t, repoDir)
	require.Len(t, events, 2)
	assert.Equal(t, "starting", events[0].Kind)
	assert.Equal(t, "[octo/app] [todo] creating label", events[0].Message)
	assert.Equal(t, "completed", events[1].Kind)
	assert.Equal(t, "[octo/app] [todo] label is created", events[1].Message)
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
	subject := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	expectedSubject := fmt.Sprintf("✅ [%s] [label] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log", "-1", "--format=%b", "state")
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
	events := eventsAtTip(t, repoDir)
	assert.Empty(t, events)
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
	events := eventsAtTip(t, repoDir)
	assert.Empty(t, events)
}

func Test_Label_should_keep_the_row_when_create_fails(t *testing.T) {
	resetWireMock(t)
	stubGet(t, "todo", http.StatusNotFound, `{"message":"Not Found"}`)
	stubPost(t, "todo", "ededed", http.StatusUnprocessableEntity, `{"message":"Validation Failed"}`)
	repoDir := setupFactory(t, []configLabel{{name: "todo", color: "ededed"}})
	id := runTick(t, repoDir)

	err := runLabel(t, repoDir, id, token)
	require.Error(t, err)

	subject := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	expectedSubject := fmt.Sprintf("❌ [%s] [label] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log", "-1", "--format=%b", "state")
	bodyText := strings.TrimSpace(string(body))
	assert.Contains(t, bodyText, "❌ [todo]")
	assert.Contains(t, bodyText, "Validation Failed")
	assert.NotContains(t, bodyText, "failed to create")

	rows := labelsAtTip(t, repoDir)
	require.Len(t, rows, 1)
	assert.Equal(t, "todo", rows[0].Name)
	assert.Equal(t, "ededed", rows[0].Color)
	assert.False(t, rows[0].CreatedAt.Valid)
	events := eventsAtTip(t, repoDir)
	require.Len(t, events, 2)
	assert.Equal(t, "starting", events[0].Kind)
	assert.Equal(t, "[octo/app] [todo] creating label", events[0].Message)
	assert.Equal(t, "failed", events[1].Kind)
	assert.Contains(t, events[1].Message, "Validation Failed")
	assert.NotContains(t, events[1].Message, "[todo]")
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
	events := eventsAtTip(t, repoDir)
	require.Len(t, events, 4)
	assert.Equal(t, "completed", events[3].Kind)
	assert.Equal(t, "[octo/app] [todo] label is created", events[3].Message)
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

	body := mustGit(t, repoDir, "log", "-1", "--format=%b", "state")
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
	events := eventsAtTip(t, repoDir)
	require.Len(t, events, 1)
	assert.Equal(t, "failed", events[0].Kind)
	assert.Contains(t, events[0].Message, "Bad credentials")
	assert.NotContains(t, events[0].Message, "[todo]")
	body := mustGit(t, repoDir, "log", "-1", "--format=%b", "state")
	assert.Contains(t, string(body), "❌ [todo]")
	assert.Contains(t, string(body), "Bad credentials")
}

func Test_Label_should_commit_when_config_has_no_labels(t *testing.T) {
	resetWireMock(t)
	repoDir := setupFactory(t, nil)
	id := runTick(t, repoDir)

	err := runLabel(t, repoDir, id, token)
	require.NoError(t, err)

	subject := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	expectedSubject := fmt.Sprintf("✅ [%s] [label] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log", "-1", "--format=%b", "state")
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

func Test_Label_should_require_a_tick_id(t *testing.T) {
	cmd := label.NewLabel()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--factory-repo", filepath.Join(JunkDir, "missing-tick")})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tick")
}

type configLabel struct {
	name  string
	color string
}

func setupFactory(t *testing.T, labels []configLabel) (repoDir string) {
	t.Helper()
	ctx := t.Context()
	repoDir = filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)

	_ = mustGit(t, repoDir, "init", "-b", "main")
	_ = mustGit(t, repoDir, "config", "user.email", "test@example.com")
	_ = mustGit(t, repoDir, "config", "user.name", "test")

	configPath := filepath.Join(repoDir, "repos", owner, repoName, "config.db")
	err = os.MkdirAll(filepath.Dir(configPath), 0755)
	require.NoError(t, err)
	configDB, err := configdb.OpenAndMigrate(ctx, configPath)
	require.NoError(t, err)
	for _, label := range labels {
		params := configrecords.CreateLabelParams{
			Name:  label.name,
			Color: label.color,
		}
		err = configDB.Queries.CreateLabel(ctx, params)
		require.NoError(t, err)
	}
	err = configDB.Close()
	require.NoError(t, err)
	configRel := filepath.Join("repos", owner, repoName, "config.db")
	_ = mustGit(t, repoDir, "add", configRel)
	_ = mustGit(t, repoDir, "commit", "-m", "config")

	_ = mustGit(t, repoDir, "checkout", "-b", "state")
	statePath := filepath.Join(repoDir, "repos", owner, repoName, "state.db")
	stateDB, err := statedb.Open(ctx, statePath)
	require.NoError(t, err)
	err = stateDB.Close()
	require.NoError(t, err)
	stateRel := filepath.Join("repos", owner, repoName, "state.db")
	_ = mustGit(t, repoDir, "add", stateRel)
	_ = mustGit(t, repoDir, "commit", "-m", "state")
	_ = mustGit(t, repoDir, "checkout", "main")
	return
}

func runTick(t *testing.T, repoDir string) (id string) {
	t.Helper()
	cmd := tick.NewTick()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--factory-repo", repoDir})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	id = strings.TrimSpace(output.String())
	require.NotEmpty(t, id)
	return
}

func runLabel(t *testing.T, repoDir string, id string, authToken string) (err error) {
	t.Helper()
	if authToken != "" {
		t.Setenv("GH_TOKEN", authToken)
	}
	cmd := label.NewLabel()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--factory-repo", repoDir, "--tick", id})
	err = cmd.ExecuteContext(t.Context())
	if err == nil {
		assert.Empty(t, output.String())
	}
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

func eventsAtTip(t *testing.T, repoDir string) (rows []records.Event) {
	t.Helper()
	db := openStateTip(t, repoDir)
	defer db.Close()
	rows, err := db.Queries.GetEvents(t.Context())
	require.NoError(t, err)
	return
}

func openStateTip(t *testing.T, repoDir string) (db *statedb.DB) {
	t.Helper()
	tipPath := filepath.Join(JunkDir, t.Name()+"-state-tip.db")
	spec := "state:repos/" + owner + "/" + repoName + "/state.db"
	blob := mustGit(t, repoDir, "show", spec)
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

func writeTokenScript(t *testing.T) (path string) {
	t.Helper()
	path = filepath.Join(JunkDir, t.Name()+"-gh")
	script := "#!/bin/sh\nif [ \"$1\" = \"auth\" ] && [ \"$2\" = \"token\" ]; then\n  echo script-token\n  exit 0\nfi\nexit 1\n"
	err := os.WriteFile(path, []byte(script), 0755)
	require.NoError(t, err)
	return
}
