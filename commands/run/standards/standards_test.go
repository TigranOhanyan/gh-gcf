package standards_test

import (
	"bytes"
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
const headSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func Test_RunStandardsReviewer_should_start_the_standards_reviewer_when_the_builder_has_finished(t *testing.T) {
	repoDir := readyBuilder(t)
	id := runTick(t, repoDir)

	err := runStandards(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [7] started")
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
	assert.True(t, strings.HasPrefix(reviewer.ID, "bc-"))
	assert.Equal(t, "standards_reviewer", reviewer.Role)
	assert.True(t, reviewer.CursorRunID.Valid)
	assert.Equal(t, runID, reviewer.CursorRunID.String)
	assert.NotEmpty(t, reviewer.StartedAt)
	assert.False(t, reviewer.FinishedAt.Valid)
	requests := cursorRequests(t)
	require.Len(t, requests, 2)
	request, found := requestFor(requests, reviewer.ID)
	require.True(t, found)
	assert.Contains(t, request, standardsInstruction)
	assert.Contains(t, request, "Pull request #9")
	assert.Contains(t, request, "Issue #7: https://github.com/octo/app/issues/7")
	assert.Contains(t, request, `"autoCreatePR":false`)
}

func Test_RunStandardsReviewer_should_record_the_standards_reviewers_started_time_and_a_starting_event(t *testing.T) {
	repoDir := readyBuilder(t)
	id := runTick(t, repoDir)
	startOfTest := time.Now().Add(-time.Second)

	err := runStandards(t, repoDir, id)
	require.NoError(t, err)
	startOfCheck := time.Now().Add(time.Second)

	db := openStateTip(t, repoDir)
	defer db.Close()
	agents := listAgents(t, db)
	reviewer, found := agentByRole(agents, "standards_reviewer")
	require.True(t, found)
	started, err := time.Parse(time.RFC3339Nano, reviewer.StartedAt)
	require.NoError(t, err)
	assert.WithinRange(t, started, startOfTest, startOfCheck)
	events := listEvents(t, db)
	starting, found := eventFor(events, reviewer.ID, "starting")
	require.True(t, found)
	assert.Len(t, starting.ID, 26)
	assert.Len(t, starting.StepID, 26)
	assert.Equal(t, reviewer.ID, starting.AgentID)
	assert.Equal(t, "starting", starting.Kind)
	assert.Equal(t, "[features] [7] starting", starting.Message)
	recorded, err := time.Parse(time.RFC3339Nano, starting.CreatedAt)
	require.NoError(t, err)
	assert.WithinRange(t, recorded, startOfTest, startOfCheck)
}

func Test_RunStandardsReviewer_should_wait_when_the_standards_reviewer_instruction_hash_differs(t *testing.T) {
	repoDir := readyBuilder(t)
	writeInstruction(t, repoDir, "standards_reviewer", "A different instruction.")
	id := runTick(t, repoDir)

	err := runStandards(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [features] instruction changed")
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.False(t, works[0].StandardsReviewerAgent.Valid)
	assert.Empty(t, works[0].Status)
	agents := listAgents(t, db)
	require.Len(t, agents, 1)
	assert.Equal(t, "builder", agents[0].Role)
	_, found := agentByRole(agents, "standards_reviewer")
	assert.False(t, found)
	assert.Len(t, cursorRequests(t), 1)
}

func Test_RunStandardsReviewer_should_not_start_a_standards_reviewer_when_the_development_work_is_attention(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubCursor(t, 200, `{"run":{"id":"`+runID+`"}}`)
	stubLabels(t, 7)
	repoDir := setupFactory(t, builderInstruction, standardsInstruction)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)
	agentID := builderAgentID(t, repoDir)
	stubRun(t, agentID, failedRun())
	failedTick := runTick(t, repoDir)
	err = runCheck(t, repoDir, failedTick)
	require.NoError(t, err)
	later := runTick(t, repoDir)

	err = runStandards(t, repoDir, later)
	require.NoError(t, err)

	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Equal(t, "attention", works[0].Status)
	assert.False(t, works[0].StandardsReviewerAgent.Valid)
	agents := listAgents(t, db)
	require.Len(t, agents, 1)
	assert.Equal(t, "builder", agents[0].Role)
	_, found := agentByRole(agents, "standards_reviewer")
	assert.False(t, found)
	assert.Len(t, cursorRequests(t), 1)
}

func readyBuilder(t *testing.T) (repoDir string) {
	t.Helper()
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubCursor(t, 200, `{"run":{"id":"`+runID+`"}}`)
	repoDir = setupFactory(t, builderInstruction, standardsInstruction)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)
	agentID := builderAgentID(t, repoDir)
	stubRun(t, agentID, finishedRun("https://github.com/octo/app/pull/9"))
	stubPull(t, 9, headSHA)
	later := runTick(t, repoDir)
	err = runCheck(t, repoDir, later)
	require.NoError(t, err)
	return
}

func setupFactory(t *testing.T, builderText string, standardsText string) (repoDir string) {
	t.Helper()
	repoDir = initFactory(t)
	mustRoot(t, "setup repo add --owner octo --name app")
	mustRoot(t, "setup label add --owner octo --repo app --name ready --color ededed")
	mustRoot(t, "setup label add --owner octo --repo app --name stop --color ff0000")
	mustRoot(t, "setup apps add --id 42")
	mustRoot(t, "setup apps attach --owner octo --repo app --id 42")
	mustRoot(t, "setup apps add --id 43")
	mustRoot(t, "setup apps attach --owner octo --repo app --id 43")
	line := fmt.Sprintf("setup development add --owner octo --repo app --name %s --starting %s --attention %s --standards-app %s --specification-app %s", developmentName, "ready", "stop", "42", "43")
	mustRoot(t, line)
	writeInstruction(t, repoDir, "builder", builderText)
	writeInstruction(t, repoDir, "standards_reviewer", standardsText)
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

func runCheck(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Setenv("GH_TOKEN", token)
	t.Chdir(repoDir)
	line := fmt.Sprintf("check builder --tick %s", id)
	_, err = runRoot(t, line)
	return
}

func runStandards(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Setenv("GH_TOKEN", token)
	t.Chdir(repoDir)
	line := fmt.Sprintf("run standards-reviewer --tick %s", id)
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

func requestFor(bodies []string, agentID string) (body string, found bool) {
	for _, item := range bodies {
		matched := strings.Contains(item, agentID)
		if matched {
			body = item
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

func cursorRequests(t *testing.T) (bodies []string) {
	t.Helper()
	journal, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	for _, entry := range journal.Requests {
		request := entry.Request
		matched := request.Method == "POST" && request.URL == "/v1/agents"
		if matched {
			bodies = append(bodies, request.Body)
		}
	}
	return
}
