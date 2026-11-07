package agent_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
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
const instruction = "Build the issue."
const builderRun = "run-11111111-1111-7111-8111-111111111111"
const builderSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const fixerSHA = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func Test_PollAgent_should_record_a_built_work(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubTimeline(t, 7, `[]`)
	stubCursorCreate(t, `{"run":{"id":"`+builderRun+`"}}`)
	repoDir := setupFactory(t, false, false)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	err = runCoding(t, repoDir, id)
	require.NoError(t, err)
	agentID := builderAgentID(t, repoDir)
	stubRun(t, agentID, builderRun, finishedRun(builderRun, agentID, "https://github.com/octo/app/pull/8"))
	stubPull(t, 8, builderSHA)
	startOfTest := time.Now().Add(-time.Second)

	err = runPoll(t, repoDir, id)
	require.NoError(t, err)
	startOfCheck := time.Now().Add(time.Second)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [7] built")
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	work := works[0]
	assert.Equal(t, int64(7), work.IssueNumber)
	assert.Equal(t, "features", work.BuilderName)
	assert.True(t, work.PullRequestNumber.Valid)
	assert.Equal(t, int64(8), work.PullRequestNumber.Int64)
	assert.True(t, work.BuilderHeadSha.Valid)
	assert.Equal(t, builderSHA, work.BuilderHeadSha.String)
	assert.False(t, work.FixerHeadSha.Valid)
	assert.Equal(t, "built", work.Status)
	events := listEvents(t, db)
	require.Len(t, events, 3)
	completed := events[2]
	assert.Equal(t, agentID, completed.AgentID)
	assert.Equal(t, "completed", completed.Kind)
	assert.Equal(t, "[7] built", completed.Message)
	assert.Len(t, completed.ID, 26)
	recorded, err := time.Parse(time.RFC3339Nano, completed.CreatedAt)
	require.NoError(t, err)
	assert.WithinRange(t, recorded, startOfTest, startOfCheck)
}

func Test_PollAgent_should_leave_a_running_agent(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubTimeline(t, 7, `[]`)
	stubCursorCreate(t, `{"run":{"id":"`+builderRun+`"}}`)
	repoDir := setupFactory(t, false, false)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	err = runCoding(t, repoDir, id)
	require.NoError(t, err)
	agentID := builderAgentID(t, repoDir)
	stubRun(t, agentID, builderRun, `{"id":"`+builderRun+`","status":"RUNNING"}`)

	err = runPoll(t, repoDir, id)
	require.NoError(t, err)

	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Equal(t, "building", works[0].Status)
	assert.False(t, works[0].PullRequestNumber.Valid)
	assert.False(t, works[0].BuilderHeadSha.Valid)
	events := listEvents(t, db)
	require.Len(t, events, 2)
	assert.Equal(t, "ack", events[1].Kind)
}

func Test_PollAgent_should_record_a_reviewer_verdict(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubTimeline(t, 7, `[]`)
	stubCursorCreate(t, `{"run":{"id":"`+builderRun+`"}}`)
	repoDir := setupFactory(t, true, false)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	err = runCoding(t, repoDir, id)
	require.NoError(t, err)
	builderID := builderAgentID(t, repoDir)
	stubRun(t, builderID, builderRun, finishedRun(builderRun, builderID, "https://github.com/octo/app/pull/8"))
	stubPull(t, 8, builderSHA)
	err = runPoll(t, repoDir, id)
	require.NoError(t, err)
	plantAgent(t, repoDir, "reviewer", "standards", "run-reviewer")
	stubRun(t, plantedID(t, repoDir, "standards"), "run-reviewer", `{"id":"run-reviewer","status":"FINISHED"}`)
	stubInstallationToken(t, 42)
	stubUser(t, 99, "standards[bot]")
	stubReviews(t, 8, `[{"id":1,"user":{"login":"standards[bot]","id":99,"type":"Bot"},"state":"APPROVED","submitted_at":"2026-04-13T18:45:00Z"}]`)
	t.Setenv("GITHUB_APP_42", testAppKey(t))

	err = runPoll(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [standards] approve")
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Equal(t, "built", works[0].Status)
	assert.Equal(t, int64(8), works[0].PullRequestNumber.Int64)
	assert.Equal(t, builderSHA, works[0].BuilderHeadSha.String)
	assert.False(t, works[0].FixerHeadSha.Valid)
	var verdict string
	err = db.DB.QueryRowContext(t.Context(), `SELECT verdict FROM reviewer_agent`).Scan(&verdict)
	require.NoError(t, err)
	assert.Equal(t, "approve", verdict)
}

func Test_PollAgent_should_record_a_fixed_work(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubTimeline(t, 7, `[]`)
	stubCursorCreate(t, `{"run":{"id":"`+builderRun+`"}}`)
	repoDir := setupFactory(t, false, true)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	err = runCoding(t, repoDir, id)
	require.NoError(t, err)
	builderID := builderAgentID(t, repoDir)
	stubRun(t, builderID, builderRun, finishedRun(builderRun, builderID, "https://github.com/octo/app/pull/8"))
	stubPull(t, 8, builderSHA)
	err = runPoll(t, repoDir, id)
	require.NoError(t, err)
	plantAgent(t, repoDir, "fixer", "patches", "run-fixer")
	resetWireMock(t)
	fixerID := plantedID(t, repoDir, "patches")
	stubRun(t, fixerID, "run-fixer", `{"id":"run-fixer","status":"FINISHED"}`)
	stubPull(t, 8, fixerSHA)

	err = runPoll(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [7] fixed")
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Equal(t, "fixed", works[0].Status)
	assert.Equal(t, builderSHA, works[0].BuilderHeadSha.String)
	assert.True(t, works[0].FixerHeadSha.Valid)
	assert.Equal(t, fixerSHA, works[0].FixerHeadSha.String)
	assert.Equal(t, int64(8), works[0].PullRequestNumber.Int64)
}

func Test_PollAgent_should_require_a_tick_id(t *testing.T) {
	_, err := runRoot(t, "poll agent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tick")
}

func setupFactory(t *testing.T, reviewer bool, fixer bool) (repoDir string) {
	t.Helper()
	repoDir = initFactory(t)
	mustRoot(t, "setup repo add --owner octo --name app")
	mustRoot(t, "setup label add --owner octo --repo app --name todo --color ededed")
	mustRoot(t, "setup label add --owner octo --repo app --name attention --color ededed")
	mustRoot(t, "setup member add-builder --owner octo --repo app --name features --starting todo --attention attention")
	writeInstruction(t, repoDir, "features", instruction)
	if reviewer {
		mustRoot(t, "setup apps add --id 42")
		mustRoot(t, "setup apps attach --owner octo --repo app --id 42")
		mustRoot(t, "setup member add-reviewer --owner octo --repo app --builder features --name standards --app 42 --attention attention")
		writeInstruction(t, repoDir, "standards", "Review the standards.")
	}
	if fixer {
		mustRoot(t, "setup member add-fixer --owner octo --repo app --builder features --name patches --attention attention")
		writeInstruction(t, repoDir, "patches", "Apply the review.")
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

func writeInstruction(t *testing.T, repoDir string, name string, text string) {
	t.Helper()
	path := filepath.Join(repoDir, "repos", owner, repoName, "members", name+".md")
	err := os.MkdirAll(filepath.Dir(path), 0755)
	require.NoError(t, err)
	err = os.WriteFile(path, []byte(text), 0644)
	require.NoError(t, err)
}

func runTick(t *testing.T, repoDir string) (id string) {
	t.Helper()
	t.Chdir(repoDir)
	stdout := mustRoot(t, "tick")
	id = strings.TrimSpace(stdout)
	require.NotEmpty(t, id)
	return
}

func runCompile(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Chdir(repoDir)
	line := fmt.Sprintf("compile coding --tick %s", id)
	_, err = runRoot(t, line)
	return
}

func runCoding(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Setenv("GH_TOKEN", token)
	t.Chdir(repoDir)
	line := fmt.Sprintf("coding --tick %s", id)
	_, err = runRoot(t, line)
	return
}

func runPoll(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Setenv("GH_TOKEN", token)
	t.Chdir(repoDir)
	line := fmt.Sprintf("poll agent --tick %s", id)
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

func builderAgentID(t *testing.T, repoDir string) (id string) {
	t.Helper()
	db := openStateTip(t, repoDir)
	defer db.Close()
	agents := listAgents(t, db)
	require.Len(t, agents, 1)
	id = agents[0].ID
	return
}

func plantedID(t *testing.T, repoDir string, name string) (id string) {
	t.Helper()
	db := openStateTip(t, repoDir)
	defer db.Close()
	err := db.DB.QueryRowContext(t.Context(), `SELECT id FROM agent WHERE member_name = ?`, name).Scan(&id)
	require.NoError(t, err)
	return
}

func plantAgent(t *testing.T, repoDir string, role string, name string, runID string) {
	t.Helper()
	_ = mustGit(t, repoDir, "checkout state")
	path := filepath.Join(repoDir, "repos", owner, repoName, "state.db")
	db, err := statedb.Open(t.Context(), path)
	require.NoError(t, err)
	var workID string
	err = db.DB.QueryRowContext(t.Context(), `SELECT id FROM coding_work`).Scan(&workID)
	require.NoError(t, err)
	var stepID string
	err = db.DB.QueryRowContext(t.Context(), `SELECT id FROM step LIMIT 1`).Scan(&stepID)
	require.NoError(t, err)
	agentID := "bc-" + name
	_, err = db.DB.ExecContext(t.Context(), `INSERT INTO agent (id, role, work_id, member_name, cursor_run_id) VALUES (?, ?, ?, ?, ?)`, agentID, role, workID, name, runID)
	require.NoError(t, err)
	table := role + "_agent"
	_, err = db.DB.ExecContext(t.Context(), `INSERT INTO `+table+` (id) VALUES (?)`, agentID)
	require.NoError(t, err)
	if role == "reviewer" {
		_, err = db.DB.ExecContext(t.Context(), `INSERT INTO github_app (app_id, client_id, installation_id) VALUES (42, 'client', 42)`)
		require.NoError(t, err)
	}
	now := time.Now()
	starting := ulid.MustNew(ulid.Timestamp(now), rand.Reader)
	acked := ulid.MustNew(ulid.Timestamp(now.Add(time.Millisecond)), rand.Reader)
	_, err = db.DB.ExecContext(t.Context(), `INSERT INTO event (id, step_id, agent_id, kind, message) VALUES (?, ?, ?, 'starting', 'starting')`, starting.String(), stepID, agentID)
	require.NoError(t, err)
	_, err = db.DB.ExecContext(t.Context(), `INSERT INTO event (id, step_id, agent_id, kind, message) VALUES (?, ?, ?, 'ack', 'ack')`, acked.String(), stepID, agentID)
	require.NoError(t, err)
	err = db.Close()
	require.NoError(t, err)
	_ = mustGit(t, repoDir, "add repos/octo/app/state.db")
	_ = mustGit(t, repoDir, "commit -m plant")
	_ = mustGit(t, repoDir, "checkout main")
}

func listWorks(t *testing.T, db *statedb.DB) (rows []records.CodingWork) {
	t.Helper()
	rows, err := db.Queries.ListCodingWorks(t.Context())
	require.NoError(t, err)
	return
}

func listAgents(t *testing.T, db *statedb.DB) (rows []records.Agent) {
	t.Helper()
	rows, err := db.Queries.ListAgents(t.Context())
	require.NoError(t, err)
	return
}

func listEvents(t *testing.T, db *statedb.DB) (rows []records.Event) {
	t.Helper()
	rows, err := db.Queries.GetEvents(t.Context())
	require.NoError(t, err)
	return
}

func issueJSON(number int, title string, body string) (payload string) {
	payload = fmt.Sprintf(`[{"number":%d,"title":"%s","body":"%s","html_url":"https://github.com/octo/app/issues/%d","state":"open"}]`, number, title, body, number)
	return
}

func finishedRun(runID string, agentID string, prURL string) (payload string) {
	payload = fmt.Sprintf(`{"id":"%s","agentId":"%s","status":"FINISHED","git":{"branches":[{"repoUrl":"github.com/octo/app","branch":"cursor/add","prUrl":"%s"}]}}`, runID, agentID, prURL)
	return
}

func resetWireMock(t *testing.T) {
	t.Helper()
	err := wiremockClient.Reset()
	require.NoError(t, err)
}

func stubIssues(t *testing.T, payload string) {
	t.Helper()
	rule := wiremock.Get(wiremock.URLPathEqualTo("/repos/octo/app/issues")).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(payload),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubTimeline(t *testing.T, number int, payload string) {
	t.Helper()
	path := fmt.Sprintf("/repos/octo/app/issues/%d/timeline", number)
	rule := wiremock.Get(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(payload),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubCursorCreate(t *testing.T, payload string) {
	t.Helper()
	rule := wiremock.Post(wiremock.URLPathEqualTo("/v1/agents")).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(payload),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubRun(t *testing.T, agentID string, runID string, payload string) {
	t.Helper()
	path := "/v1/agents/" + agentID + "/runs/" + runID
	rule := wiremock.Get(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(payload),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubPull(t *testing.T, number int, sha string) {
	t.Helper()
	path := fmt.Sprintf("/repos/octo/app/pulls/%d", number)
	payload := fmt.Sprintf(`{"number":%d,"head":{"sha":"%s","ref":"cursor/add"},"base":{"sha":"cccccccccccccccccccccccccccccccccccccccc","ref":"main"}}`, number, sha)
	rule := wiremock.Get(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(payload),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubReviews(t *testing.T, number int, payload string) {
	t.Helper()
	path := fmt.Sprintf("/repos/octo/app/pulls/%d/reviews", number)
	rule := wiremock.Get(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(payload),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubInstallationToken(t *testing.T, id int) {
	t.Helper()
	path := fmt.Sprintf("/app/installations/%d/access_tokens", id)
	rule := wiremock.Post(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().WithStatus(201).WithHeader("Content-Type", "application/json").WithBody(`{"token":"ghs_test","expires_at":"2030-01-01T00:00:00Z"}`),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubUser(t *testing.T, id int, login string) {
	t.Helper()
	payload := fmt.Sprintf(`{"id":%d,"login":"%s"}`, id, login)
	rule := wiremock.Get(wiremock.URLPathEqualTo("/user")).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(payload),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func testAppKey(t *testing.T) (pemText string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}
	pemText = string(pem.EncodeToMemory(block))
	return
}
