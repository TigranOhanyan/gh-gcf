package app_test

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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wiremock/go-wiremock"

	"github.com/TigranOhanyan/gh-gcf/commands/app"
	"github.com/TigranOhanyan/gh-gcf/commands/tick"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const owner = "octo"
const repoName = "app"
const appID int64 = 123
const clientID = "Iv1.abc"

func Test_App_should_record_an_installation(t *testing.T) {
	resetWireMock(t)
	stubInstallation(t, 42, clientID, "")
	repoDir := setupFactory(t, []int64{appID})
	id := runTick(t, repoDir)
	pemText := testAppKey(t)
	t.Setenv("GITHUB_APP_123", pemText)

	err := runApp(t, repoDir, id)
	require.NoError(t, err)

	requireJWT(t, "GET", "/repos/octo/app/installation")
	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("✅ [%s] [app] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Equal(t, "✅ [123] recorded", strings.TrimSpace(string(body)))

	rows := appsAtTip(t, repoDir)
	require.Len(t, rows, 1)
	assert.Equal(t, appID, rows[0].AppID)
	assert.Equal(t, clientID, rows[0].ClientID)
	assert.Equal(t, int64(42), rows[0].InstallationID)
}

func Test_App_should_leave_a_matching_installation(t *testing.T) {
	resetWireMock(t)
	stubInstallation(t, 42, clientID, "")
	repoDir := setupFactory(t, []int64{appID})
	id := runTick(t, repoDir)
	pemText := testAppKey(t)
	t.Setenv("GITHUB_APP_123", pemText)
	err := runApp(t, repoDir, id)
	require.NoError(t, err)
	resetWireMock(t)
	stubInstallation(t, 42, clientID, "")

	err = runApp(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Empty(t, strings.TrimSpace(string(body)))
	rows := appsAtTip(t, repoDir)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(42), rows[0].InstallationID)
	assert.Equal(t, clientID, rows[0].ClientID)
}

func Test_App_should_update_a_changed_installation(t *testing.T) {
	resetWireMock(t)
	stubInstallation(t, 42, clientID, "")
	repoDir := setupFactory(t, []int64{appID})
	id := runTick(t, repoDir)
	pemText := testAppKey(t)
	t.Setenv("GITHUB_APP_123", pemText)
	err := runApp(t, repoDir, id)
	require.NoError(t, err)
	resetWireMock(t)
	stubInstallation(t, 77, "Iv1.next", "")

	err = runApp(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Equal(t, "✅ [123] installation updated", strings.TrimSpace(string(body)))
	rows := appsAtTip(t, repoDir)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(77), rows[0].InstallationID)
	assert.Equal(t, "Iv1.next", rows[0].ClientID)
}

func Test_App_should_refuse_a_missing_installation(t *testing.T) {
	resetWireMock(t)
	stubMissingInstallation(t)
	repoDir := setupFactory(t, []int64{appID})
	id := runTick(t, repoDir)
	pemText := testAppKey(t)
	t.Setenv("GITHUB_APP_123", pemText)

	err := runApp(t, repoDir, id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "installation is missing")

	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("❌ [%s] [app] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	rows := appsAtTip(t, repoDir)
	assert.Empty(t, rows)
}

func Test_App_should_refuse_a_suspended_installation_and_keep_the_row(t *testing.T) {
	resetWireMock(t)
	stubInstallation(t, 42, clientID, "")
	repoDir := setupFactory(t, []int64{appID})
	id := runTick(t, repoDir)
	pemText := testAppKey(t)
	t.Setenv("GITHUB_APP_123", pemText)
	err := runApp(t, repoDir, id)
	require.NoError(t, err)
	resetWireMock(t)
	stubInstallation(t, 42, clientID, "2020-01-01T00:00:00Z")

	err = runApp(t, repoDir, id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "installation is suspended")

	rows := appsAtTip(t, repoDir)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(42), rows[0].InstallationID)
	assert.Equal(t, clientID, rows[0].ClientID)
}

func Test_App_should_refuse_a_missing_key_and_write_nothing(t *testing.T) {
	resetWireMock(t)
	stubInstallation(t, 42, clientID, "")
	repoDir := setupFactory(t, []int64{appID})
	id := runTick(t, repoDir)
	err := os.Unsetenv("GITHUB_APP_123")
	require.NoError(t, err)

	err = runApp(t, repoDir, id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github app key is missing")

	journal, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	assert.Empty(t, journal.Requests)
	rows := appsAtTip(t, repoDir)
	assert.Empty(t, rows)
}

func Test_App_should_leave_an_app_that_config_lacks(t *testing.T) {
	resetWireMock(t)
	repoDir := setupFactory(t, nil)
	storeApp(t, repoDir, 9, "Iv1.old", 3)
	id := runTick(t, repoDir)

	err := runApp(t, repoDir, id)
	require.NoError(t, err)

	journal, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	assert.Empty(t, journal.Requests)
	rows := appsAtTip(t, repoDir)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(9), rows[0].AppID)
	assert.Equal(t, "Iv1.old", rows[0].ClientID)
	assert.Equal(t, int64(3), rows[0].InstallationID)
}

func Test_App_should_refuse_a_missing_tick(t *testing.T) {
	cmd := app.NewApp()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tick")
}

func setupFactory(t *testing.T, ids []int64) (repoDir string) {
	t.Helper()
	ctx := t.Context()
	repoDir = filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)

	_ = mustGit(t, repoDir, "init -b main")
	_ = mustGit(t, repoDir, "config user.email test@example.com")
	_ = mustGit(t, repoDir, "config user.name test")

	configPath := filepath.Join(repoDir, "repos", owner, repoName, "config.db")
	err = os.MkdirAll(filepath.Dir(configPath), 0755)
	require.NoError(t, err)
	configDB, err := configdb.OpenAndMigrate(ctx, configPath)
	require.NoError(t, err)
	for _, id := range ids {
		err = configDB.Queries.CreateGithubApp(ctx, id)
		require.NoError(t, err)
	}
	err = configDB.Close()
	require.NoError(t, err)
	configRel := filepath.Join("repos", owner, repoName, "config.db")
	addLine := fmt.Sprintf("add %s", configRel)
	_ = mustGit(t, repoDir, addLine)
	_ = mustGit(t, repoDir, "commit -m config")

	_ = mustGit(t, repoDir, "checkout -b state")
	statePath := filepath.Join(repoDir, "repos", owner, repoName, "state.db")
	stateDB, err := statedb.Open(ctx, statePath)
	require.NoError(t, err)
	err = stateDB.Close()
	require.NoError(t, err)
	stateRel := filepath.Join("repos", owner, repoName, "state.db")
	stateLine := fmt.Sprintf("add %s", stateRel)
	_ = mustGit(t, repoDir, stateLine)
	_ = mustGit(t, repoDir, "commit -m state")
	_ = mustGit(t, repoDir, "checkout main")
	return
}

func storeApp(t *testing.T, repoDir string, id int64, client string, installationID int64) {
	t.Helper()
	_ = mustGit(t, repoDir, "checkout state")
	path := filepath.Join(repoDir, "repos", owner, repoName, "state.db")
	db, err := statedb.Open(t.Context(), path)
	require.NoError(t, err)
	params := records.CreateGithubAppParams{
		AppID:          id,
		ClientID:       client,
		InstallationID: installationID,
	}
	err = db.Queries.CreateGithubApp(t.Context(), params)
	require.NoError(t, err)
	err = db.Close()
	require.NoError(t, err)
	rel := filepath.Join("repos", owner, repoName, "state.db")
	addLine := fmt.Sprintf("add %s", rel)
	_ = mustGit(t, repoDir, addLine)
	_ = mustGit(t, repoDir, "commit -m app")
	_ = mustGit(t, repoDir, "checkout main")
}

func runTick(t *testing.T, repoDir string) (id string) {
	t.Helper()
	cmd := tick.NewTick()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	t.Chdir(repoDir)

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	id = strings.TrimSpace(output.String())
	require.NotEmpty(t, id)
	return
}

func runApp(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	cmd := app.NewApp()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	t.Chdir(repoDir)
	cmd.SetArgs([]string{"--tick", id})
	err = cmd.ExecuteContext(t.Context())
	return
}

func appsAtTip(t *testing.T, repoDir string) (rows []records.GithubApp) {
	t.Helper()
	tipPath := filepath.Join(JunkDir, t.Name()+"-state-tip.db")
	spec := "state:repos/" + owner + "/" + repoName + "/state.db"
	showLine := fmt.Sprintf("show %s", spec)
	blob := mustGit(t, repoDir, showLine)
	err := os.WriteFile(tipPath, blob, 0644)
	require.NoError(t, err)
	db, err := statedb.Open(t.Context(), tipPath)
	require.NoError(t, err)
	defer db.Close()
	rows, err = db.Queries.ListGithubApps(t.Context())
	require.NoError(t, err)
	return
}

func resetWireMock(t *testing.T) {
	t.Helper()
	err := wiremockClient.Reset()
	require.NoError(t, err)
}

func stubInstallation(t *testing.T, id int64, client string, suspendedAt string) {
	t.Helper()
	path := fmt.Sprintf("/repos/%s/%s/installation", owner, repoName)
	body := fmt.Sprintf(`{"id":%d,"client_id":%q,"suspended_at":null}`, id, client)
	hasSuspended := suspendedAt != ""
	if hasSuspended {
		body = fmt.Sprintf(`{"id":%d,"client_id":%q,"suspended_at":%q}`, id, client, suspendedAt)
	}
	rule := wiremock.Get(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().
			WithStatus(int64(http.StatusOK)).
			WithHeader("Content-Type", "application/json").
			WithBody(body),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubMissingInstallation(t *testing.T) {
	t.Helper()
	path := fmt.Sprintf("/repos/%s/%s/installation", owner, repoName)
	rule := wiremock.Get(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().
			WithStatus(int64(http.StatusNotFound)).
			WithHeader("Content-Type", "application/json").
			WithBody(`{"message":"Not Found"}`),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
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
