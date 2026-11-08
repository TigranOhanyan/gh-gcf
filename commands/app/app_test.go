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

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
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
	stubInstallation(t, 3, "Iv1.old", "")
	repoDir := setupFactory(t, []int64{9})
	id := runTick(t, repoDir)
	pemText := testAppKey(t)
	t.Setenv("GITHUB_APP_9", pemText)
	err := runApp(t, repoDir, id)
	require.NoError(t, err)
	dropConfigApp(t, repoDir)
	resetWireMock(t)

	err = runApp(t, repoDir, id)
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
	_, err := runRoot(t, "app")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tick")
}

func setupFactory(t *testing.T, ids []int64) (repoDir string) {
	t.Helper()
	repoDir = initFactory(t)
	mustRoot(t, "setup repo add --owner octo --name app")
	for _, id := range ids {
		text := fmt.Sprintf("%d", id)
		addLine := "setup apps add --id " + text
		mustRoot(t, addLine)
		attachLine := "setup apps attach --owner octo --repo app --id " + text
		mustRoot(t, attachLine)
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

func dropConfigApp(t *testing.T, repoDir string) {
	t.Helper()
	path := filepath.Join(repoDir, "repos", owner, repoName, "config.db")
	db, err := configdb.OpenAndMigrate(t.Context(), path)
	require.NoError(t, err)
	_, err = db.DB.ExecContext(t.Context(), `DELETE FROM github_app`)
	require.NoError(t, err)
	err = db.Close()
	require.NoError(t, err)
	rel := filepath.Join("repos", owner, repoName, "config.db")
	addLine := fmt.Sprintf("add %s", rel)
	_ = mustGit(t, repoDir, addLine)
	_ = mustGit(t, repoDir, "commit -m config")
}

func runTick(t *testing.T, repoDir string) (id string) {
	t.Helper()
	t.Chdir(repoDir)
	stdout := mustRoot(t, "run tick")
	id = strings.TrimSpace(stdout)
	require.NotEmpty(t, id)
	return
}

func runApp(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Chdir(repoDir)
	line := fmt.Sprintf("app --tick %s", id)
	_, err = runRoot(t, line)
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
