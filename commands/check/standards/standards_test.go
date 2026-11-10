package standards_test

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
const developmentName = "features"
const builderInstruction = "Build the issue."
const standardsInstruction = "Review the standards."
const runID = "run-11111111-1111-7111-8111-111111111111"
const cursorAgentID = "agent-11111111-1111-7111-8111-111111111111"
const headSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const standardsInstallation int64 = 420
const standardsToken = "ghs_standards"

func Test_CheckStandardsReviewer_should_record_the_standards_reviewers_finished_time(t *testing.T) {
	repoDir := startedStandardsReviewer(t)
	reviewerID := cursorAgentIDByRole(t, repoDir, "standards_reviewer")
	stubRun(t, reviewerID, finishedReviewerRun())
	later := runTick(t, repoDir)
	pulls := len(pullGets(t))
	startOfTest := time.Now().Add(-time.Second)

	err := runCheck(t, repoDir, later)
	require.NoError(t, err)
	startOfCheck := time.Now().Add(time.Second)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [7] standards reviewed")
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	work := works[0]
	agents := listAgents(t, db)
	require.Len(t, agents, 2)
	builderAgent, builderFound := agentByRole(agents, "builder")
	require.True(t, builderFound)
	reviewer, reviewerFound := agentByRole(agents, "standards_reviewer")
	require.True(t, reviewerFound)
	assert.NotEmpty(t, work.ID)
	assert.Equal(t, developmentName, work.DevelopmentName)
	assert.Equal(t, int64(7), work.IssueNumber)
	assert.Equal(t, "https://github.com/octo/app/issues/7", work.IssueUrl)
	assert.True(t, work.PullRequestNumber.Valid)
	assert.Equal(t, int64(9), work.PullRequestNumber.Int64)
	assert.True(t, work.BuilderHeadSha.Valid)
	assert.Equal(t, headSHA, work.BuilderHeadSha.String)
	assert.False(t, work.FixerHeadSha.Valid)
	assert.True(t, work.BuilderAgent.Valid)
	assert.Equal(t, builderAgent.ID, work.BuilderAgent.String)
	assert.True(t, work.StandardsReviewerAgent.Valid)
	assert.Equal(t, reviewer.ID, work.StandardsReviewerAgent.String)
	assert.False(t, work.SpecificationReviewerAgent.Valid)
	assert.False(t, work.FixerAgent.Valid)
	assert.Empty(t, work.Status)
	assert.True(t, builderAgent.FinishedAt.Valid)
	assert.Equal(t, "standards_reviewer", reviewer.Role)
	assert.True(t, reviewer.CursorRunID.Valid)
	assert.Equal(t, runID, reviewer.CursorRunID.String)
	assert.NotEmpty(t, reviewer.StartedAt)
	assert.True(t, reviewer.FinishedAt.Valid)
	finished, err := time.Parse(time.RFC3339Nano, reviewer.FinishedAt.String)
	require.NoError(t, err)
	assert.WithinRange(t, finished, startOfTest, startOfCheck)
	events := listEvents(t, db)
	completed, found := eventFor(events, reviewer.ID, "completed")
	require.True(t, found)
	assert.Len(t, completed.ID, 26)
	assert.Len(t, completed.StepID, 26)
	assert.Equal(t, reviewer.ID, completed.AgentID)
	assert.Equal(t, "completed", completed.Kind)
	assert.Equal(t, "[7] standards reviewed", completed.Message)
	recorded, err := time.Parse(time.RFC3339Nano, completed.CreatedAt)
	require.NoError(t, err)
	assert.WithinRange(t, recorded, startOfTest, startOfCheck)
	assert.Len(t, pullGets(t), pulls)
}

func Test_CheckStandardsReviewer_should_leave_a_running_standards_reviewer_and_write_no_finished_time(t *testing.T) {
	repoDir := startedStandardsReviewer(t)
	reviewerID := cursorAgentIDByRole(t, repoDir, "standards_reviewer")
	stubRun(t, reviewerID, runningRun())
	later := runTick(t, repoDir)
	runs := len(cursorRunRequests(t))

	err := runCheck(t, repoDir, later)
	require.NoError(t, err)

	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Empty(t, works[0].Status)
	agents := listAgents(t, db)
	reviewer, found := agentByRole(agents, "standards_reviewer")
	require.True(t, found)
	assert.False(t, reviewer.FinishedAt.Valid)
	events := listEvents(t, db)
	_, completed := eventFor(events, reviewer.ID, "completed")
	assert.False(t, completed)
	_, failed := eventFor(events, reviewer.ID, "failed")
	assert.False(t, failed)
	assert.Len(t, cursorRunRequests(t), runs+1)
}

func Test_CheckStandardsReviewer_should_set_attention_when_the_standards_reviewer_run_has_failed(t *testing.T) {
	repoDir := startedStandardsReviewer(t)
	reviewerID := cursorAgentIDByRole(t, repoDir, "standards_reviewer")
	stubRun(t, reviewerID, failedRun())
	stubLabels(t, 7)
	later := runTick(t, repoDir)

	err := runCheck(t, repoDir, later)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [7] ERROR")
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Equal(t, "attention", works[0].Status)
	agents := listAgents(t, db)
	reviewer, found := agentByRole(agents, "standards_reviewer")
	require.True(t, found)
	assert.False(t, reviewer.FinishedAt.Valid)
	events := listEvents(t, db)
	failed, found := eventFor(events, reviewer.ID, "failed")
	require.True(t, found)
	assert.Equal(t, "[7] ERROR", failed.Message)
	_, completed := eventFor(events, reviewer.ID, "completed")
	assert.False(t, completed)
	labels := labelPosts(t)
	require.Len(t, labels, 1)
	assert.Equal(t, "/repos/octo/app/issues/7/labels", labels[0].url)
	assert.Contains(t, labels[0].body, "stop")
}

func Test_CheckStandardsReviewer_should_ask_cursor_with_the_stored_cursor_agent_id(t *testing.T) {
	repoDir := startedStandardsReviewer(t)
	stored := cursorAgentIDByRole(t, repoDir, "standards_reviewer")
	stubRun(t, stored, finishedReviewerRun())
	later := runTick(t, repoDir)

	err := runCheck(t, repoDir, later)
	require.NoError(t, err)

	want := fmt.Sprintf("/v1/agents/%s/runs/%s", stored, runID)
	assert.Contains(t, cursorRunRequests(t), want)
}

func Test_CheckStandardsReviewer_should_skip_an_agent_that_has_no_cursor_agent_id(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	created := fmt.Sprintf(`{"agent":{"id":"%s"},"run":{"id":"%s"}}`, cursorAgentID, runID)
	stubCursor(t, 200, created)
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)
	agentID := builderAgentID(t, repoDir)
	stubRun(t, agentID, finishedRun("https://github.com/octo/app/pull/9"))
	stubPull(t, 9, headSHA)
	later := runTick(t, repoDir)
	err = runCheckBuilder(t, repoDir, later)
	require.NoError(t, err)
	resetWireMock(t)
	stubCursorFault(t)
	standardsTick := runTick(t, repoDir)
	err = runStandards(t, repoDir, standardsTick)
	require.Error(t, err)
	checked := runTick(t, repoDir)
	before := len(cursorRunRequests(t))

	err = runCheck(t, repoDir, checked)
	require.NoError(t, err)

	assert.Len(t, cursorRunRequests(t), before)
	db := openStateTip(t, repoDir)
	defer db.Close()
	agents := listAgents(t, db)
	reviewer, found := agentByRole(agents, "standards_reviewer")
	require.True(t, found)
	assert.False(t, reviewer.CursorAgentID.Valid)
	assert.Empty(t, worksStatus(t, db))
}

func worksStatus(t *testing.T, db *statedb.DB) (status string) {
	t.Helper()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	status = works[0].Status
	return
}

func Test_CheckStandardsReviewer_should_leave_an_attention_work(t *testing.T) {
	repoDir := startedStandardsReviewer(t)
	reviewerID := cursorAgentIDByRole(t, repoDir, "standards_reviewer")
	stubRun(t, reviewerID, failedRun())
	stubLabels(t, 7)
	failedTick := runTick(t, repoDir)
	err := runCheck(t, repoDir, failedTick)
	require.NoError(t, err)
	later := runTick(t, repoDir)

	err = runCheck(t, repoDir, later)
	require.NoError(t, err)
	err = runStandards(t, repoDir, later)
	require.NoError(t, err)

	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Equal(t, "attention", works[0].Status)
	agents := listAgents(t, db)
	require.Len(t, agents, 2)
	reviewer, found := agentByRole(agents, "standards_reviewer")
	require.True(t, found)
	assert.False(t, reviewer.FinishedAt.Valid)
	events := listEvents(t, db)
	_, failed := eventFor(events, reviewer.ID, "failed")
	assert.True(t, failed)
	assert.Len(t, cursorCreates(t), 2)
	assert.Len(t, labelPosts(t), 1)
}

func startedStandardsReviewer(t *testing.T) (repoDir string) {
	t.Helper()
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	created := fmt.Sprintf(`{"agent":{"id":"%s"},"run":{"id":"%s"}}`, cursorAgentID, runID)
	stubCursor(t, 200, created)
	repoDir = setupFactory(t)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)
	agentID := builderAgentID(t, repoDir)
	stubRun(t, agentID, finishedRun("https://github.com/octo/app/pull/9"))
	stubPull(t, 9, headSHA)
	later := runTick(t, repoDir)
	err = runCheckBuilder(t, repoDir, later)
	require.NoError(t, err)
	standardsTick := runTick(t, repoDir)
	err = runStandards(t, repoDir, standardsTick)
	require.NoError(t, err)
	return
}

func setupFactory(t *testing.T) (repoDir string) {
	t.Helper()
	repoDir = initFactory(t)
	mustRoot(t, "setup repo add --owner octo --name app")
	mustRoot(t, "setup label add --owner octo --repo app --name ready --color ededed")
	mustRoot(t, "setup label add --owner octo --repo app --name stop --color ff0000")
	mustRoot(t, "setup apps add --id 42")
	mustRoot(t, "setup apps attach --owner octo --repo app --id 42")
	mustRoot(t, "setup apps add --id 43")
	mustRoot(t, "setup apps attach --owner octo --repo app --id 43")
	mustRoot(t, "setup apps add --id 44")
	mustRoot(t, "setup apps attach --owner octo --repo app --id 44")
	line := fmt.Sprintf("setup development add --owner octo --repo app --name %s --starting %s --attention %s --standards-app %s --specification-app %s --builder-app %s --fixer-app %s", developmentName, "ready", "stop", "42", "43", "44", "44")
	mustRoot(t, line)
	writeInstruction(t, repoDir, "builder", builderInstruction)
	writeInstruction(t, repoDir, "standards_reviewer", standardsInstruction)
	writeInstruction(t, repoDir, "specification_reviewer", "")
	writeInstruction(t, repoDir, "fixer", "")
	commitConfig(t, repoDir)
	useStateBranch(t, repoDir)
	return
}

func writeInstruction(t *testing.T, repoDir string, role string, text string) {
	t.Helper()
	path := filepath.Join(repoDir, "repos", owner, repoName, "developments", developmentName, role+".md")
	err := os.MkdirAll(filepath.Dir(path), 0755)
	require.NoError(t, err)
	err = os.WriteFile(path, []byte(text), 0644)
	require.NoError(t, err)
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
	stdout := mustRoot(t, "run tick")
	id = strings.TrimSpace(stdout)
	require.NotEmpty(t, id)
	return
}

func runCompile(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Chdir(repoDir)
	line := fmt.Sprintf("compile development --tick %s", id)
	_, err = runRoot(t, line)
	return
}

func runBuilder(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Setenv("GH_TOKEN", token)
	t.Chdir(repoDir)
	line := fmt.Sprintf("run builder --tick %s", id)
	_, err = runRoot(t, line)
	return
}

func runCheckBuilder(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Setenv("GH_TOKEN", token)
	t.Chdir(repoDir)
	line := fmt.Sprintf("check builder --tick %s", id)
	_, err = runRoot(t, line)
	return
}

func runStandards(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	allowStandardsApp(t)
	t.Setenv("GH_TOKEN", token)
	t.Chdir(repoDir)
	line := fmt.Sprintf("run standards-reviewer --tick %s", id)
	_, err = runRoot(t, line)
	return
}

func allowStandardsApp(t *testing.T) {
	t.Helper()
	pemText := testAppKey(t)
	t.Setenv("FACTORY_GITHUB_APP_42", pemText)
	stubInstallation(t, standardsInstallation)
	stubInstallationToken(t, standardsInstallation, standardsToken)
}

func stubInstallation(t *testing.T, installationID int64) {
	t.Helper()
	path := fmt.Sprintf("/repos/%s/%s/installation", owner, repoName)
	body := fmt.Sprintf(`{"id":%d,"client_id":"Iv1.standards","suspended_at":null}`, installationID)
	rule := wiremock.Get(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(body),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubInstallationToken(t *testing.T, installationID int64, issued string) {
	t.Helper()
	path := fmt.Sprintf("/app/installations/%d/access_tokens", installationID)
	body := fmt.Sprintf(`{"token":%q}`, issued)
	rule := wiremock.Post(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(body),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubCursorFault(t *testing.T) {
	t.Helper()
	rule := wiremock.Post(wiremock.URLPathEqualTo("/v1/agents")).WillReturnResponse(
		wiremock.NewResponse().WithFault(wiremock.FaultConnectionResetByPeer),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
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

func runCheck(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Setenv("GH_TOKEN", token)
	t.Chdir(repoDir)
	line := fmt.Sprintf("check standards-reviewer --tick %s", id)
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

func builderAgentID(t *testing.T, repoDir string) (id string) {
	t.Helper()
	db := openStateTip(t, repoDir)
	defer db.Close()
	agents := listAgents(t, db)
	require.Len(t, agents, 1)
	id = agents[0].ID
	return
}

func cursorAgentIDByRole(t *testing.T, repoDir string, role string) (id string) {
	t.Helper()
	db := openStateTip(t, repoDir)
	defer db.Close()
	agents := listAgents(t, db)
	agent, found := agentByRole(agents, role)
	require.True(t, found)
	require.True(t, agent.CursorAgentID.Valid)
	id = agent.CursorAgentID.String
	return
}

func openStateTip(t *testing.T, repoDir string) (db *statedb.DB) {
	t.Helper()
	tipPath := filepath.Join(JunkDir, t.Name()+"-state-tip.db")
	spec := fmt.Sprintf("state:repos/%s/%s/state.db", owner, repoName)
	showLine := fmt.Sprintf("show %s", spec)
	blob := mustGit(t, repoDir, showLine)
	err := os.WriteFile(tipPath, blob, 0644)
	require.NoError(t, err)
	db, err = statedb.Open(t.Context(), tipPath)
	require.NoError(t, err)
	return
}

func listWorks(t *testing.T, db *statedb.DB) (rows []records.DevelopmentWork) {
	t.Helper()
	rows, err := db.Queries.ListDevelopmentWorks(t.Context())
	require.NoError(t, err)
	return
}

func listAgents(t *testing.T, db *statedb.DB) (rows []records.DevelopmentAgent) {
	t.Helper()
	rows, err := db.Queries.ListDevelopmentAgents(t.Context())
	require.NoError(t, err)
	return
}

func listEvents(t *testing.T, db *statedb.DB) (rows []records.DevelopmentEvent) {
	t.Helper()
	rows, err := db.Queries.ListDevelopmentEvents(t.Context())
	require.NoError(t, err)
	return
}

func agentByRole(agents []records.DevelopmentAgent, role string) (agent records.DevelopmentAgent, found bool) {
	for _, item := range agents {
		matched := item.Role == role
		if matched {
			agent = item
			found = true
			return
		}
	}
	return
}

func eventFor(events []records.DevelopmentEvent, agentID string, kind string) (event records.DevelopmentEvent, found bool) {
	for _, item := range events {
		matched := item.AgentID == agentID && item.Kind == kind
		if matched {
			event = item
			found = true
			return
		}
	}
	return
}

func issueJSON(number int, title string, body string) (payload string) {
	payload = fmt.Sprintf(`[{"number":%d,"title":"%s","body":"%s","html_url":"https://github.com/octo/app/issues/%d","state":"open"}]`, number, title, body, number)
	return
}

func finishedRun(prURL string) (payload string) {
	payload = fmt.Sprintf(`{"id":"%s","status":"FINISHED","git":{"branches":[{"repoUrl":"https://github.com/octo/app","prUrl":"%s"}]}}`, runID, prURL)
	return
}

func finishedReviewerRun() (payload string) {
	payload = fmt.Sprintf(`{"id":"%s","status":"FINISHED"}`, runID)
	return
}

func runningRun() (payload string) {
	payload = fmt.Sprintf(`{"id":"%s","status":"RUNNING"}`, runID)
	return
}

func failedRun() (payload string) {
	payload = fmt.Sprintf(`{"id":"%s","status":"ERROR"}`, runID)
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

func stubCursor(t *testing.T, status int, payload string) {
	t.Helper()
	rule := wiremock.Post(wiremock.URLPathEqualTo("/v1/agents")).WillReturnResponse(
		wiremock.NewResponse().WithStatus(int64(status)).WithHeader("Content-Type", "application/json").WithBody(payload),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubRun(t *testing.T, agentID string, payload string) {
	t.Helper()
	path := fmt.Sprintf("/v1/agents/%s/runs/%s", agentID, runID)
	rule := wiremock.Get(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(payload),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubPull(t *testing.T, number int, sha string) {
	t.Helper()
	path := fmt.Sprintf("/repos/octo/app/pulls/%d", number)
	body := fmt.Sprintf(`{"number":%d,"head":{"sha":"%s"}}`, number, sha)
	rule := wiremock.Get(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(body),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubLabels(t *testing.T, number int) {
	t.Helper()
	path := fmt.Sprintf("/repos/octo/app/issues/%d/labels", number)
	rule := wiremock.Post(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(`[]`),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func cursorCreates(t *testing.T) (paths []string) {
	t.Helper()
	journal, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	for _, entry := range journal.Requests {
		request := entry.Request
		matched := request.Method == "POST" && request.URL == "/v1/agents"
		if matched {
			paths = append(paths, request.URL)
		}
	}
	return
}

func cursorRunRequests(t *testing.T) (paths []string) {
	t.Helper()
	journal, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	for _, entry := range journal.Requests {
		request := entry.Request
		matched := request.Method == "GET" && strings.Contains(request.URL, "/runs/")
		if matched {
			paths = append(paths, request.URL)
		}
	}
	return
}

func pullGets(t *testing.T) (paths []string) {
	t.Helper()
	journal, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	for _, entry := range journal.Requests {
		request := entry.Request
		matched := request.Method == "GET" && strings.Contains(request.URL, "/pulls/")
		if matched {
			paths = append(paths, request.URL)
		}
	}
	return
}

type labelPost struct {
	url  string
	body string
}

func labelPosts(t *testing.T) (posts []labelPost) {
	t.Helper()
	journal, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	for _, entry := range journal.Requests {
		request := entry.Request
		matched := request.Method == "POST" && strings.HasSuffix(request.URL, "/labels")
		if matched {
			post := labelPost{url: request.URL, body: request.Body}
			posts = append(posts, post)
		}
	}
	return
}
