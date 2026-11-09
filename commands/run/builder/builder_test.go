package builder_test

import (
	"bytes"
	"database/sql"
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
const instruction = "Build the issue."
const runID = "run-11111111-1111-7111-8111-111111111111"

func Test_RunBuilder_should_start_a_builder_for_an_open_issue(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubCursor(t, 200, `{"run":{"id":"`+runID+`"}}`)
	repoDir := setupFactory(t, instruction)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)

	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [7] started")
	db := openStateTip(t, repoDir)
	defer db.Close()
	agents := listAgents(t, db)
	require.Len(t, agents, 1)
	assert.True(t, strings.HasPrefix(agents[0].ID, "bc-"))
	assert.Equal(t, "builder", agents[0].Role)
	assert.True(t, agents[0].CursorRunID.Valid)
	assert.Equal(t, runID, agents[0].CursorRunID.String)
	requests := cursorRequests(t)
	require.Len(t, requests, 1)
	assert.Contains(t, requests[0], instruction)
	assert.Contains(t, requests[0], "Issue #7: https://github.com/octo/app/issues/7")
	assert.NotContains(t, requests[0], "Add a thing")
	assert.NotContains(t, requests[0], "Do it")
	assert.Contains(t, requests[0], agents[0].ID)
}

func Test_RunBuilder_should_start_a_builder_without_writing_a_coding_work(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubCursor(t, 200, `{"run":{"id":"`+runID+`"}}`)
	repoDir := setupFactory(t, instruction)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)

	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)

	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Equal(t, int64(7), works[0].IssueNumber)
	assertAbsentTable(t, db.DB, "coding_work")
}

func Test_RunBuilder_should_store_the_issue_and_point_at_the_development_and_the_builder(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubCursor(t, 200, `{"run":{"id":"`+runID+`"}}`)
	repoDir := setupFactory(t, instruction)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)

	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)

	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	work := works[0]
	agents := listAgents(t, db)
	require.Len(t, agents, 1)
	assert.NotEmpty(t, work.ID)
	assert.Equal(t, developmentName, work.DevelopmentName)
	assert.Equal(t, int64(7), work.IssueNumber)
	assert.Equal(t, "https://github.com/octo/app/issues/7", work.IssueUrl)
	assert.False(t, work.PullRequestNumber.Valid)
	assert.False(t, work.BuilderHeadSha.Valid)
	assert.False(t, work.FixerHeadSha.Valid)
	assert.True(t, work.BuilderAgent.Valid)
	assert.Equal(t, agents[0].ID, work.BuilderAgent.String)
	assert.False(t, work.StandardsReviewerAgent.Valid)
	assert.False(t, work.SpecificationReviewerAgent.Valid)
	assert.False(t, work.FixerAgent.Valid)
	assert.Empty(t, work.Status)
}

func Test_RunBuilder_should_record_the_builder_started_time_and_a_starting_event(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubCursor(t, 200, `{"run":{"id":"`+runID+`"}}`)
	repoDir := setupFactory(t, instruction)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	startOfTest := time.Now().Add(-time.Second)

	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)
	startOfCheck := time.Now().Add(time.Second)

	db := openStateTip(t, repoDir)
	defer db.Close()
	agents := listAgents(t, db)
	require.Len(t, agents, 1)
	started, err := time.Parse(time.RFC3339Nano, agents[0].StartedAt)
	require.NoError(t, err)
	assert.WithinRange(t, started, startOfTest, startOfCheck)
	events := listEvents(t, db)
	require.NotEmpty(t, events)
	starting := events[0]
	assert.Len(t, starting.ID, 26)
	assert.Len(t, starting.StepID, 26)
	assert.Equal(t, agents[0].ID, starting.AgentID)
	assert.Equal(t, "starting", starting.Kind)
	assert.Equal(t, "[features] [7] starting", starting.Message)
	recorded, err := time.Parse(time.RFC3339Nano, starting.CreatedAt)
	require.NoError(t, err)
	assert.WithinRange(t, recorded, startOfTest, startOfCheck)
}

func Test_RunBuilder_should_leave_an_issue_that_already_has_a_development_work(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubCursor(t, 200, `{"run":{"id":"`+runID+`"}}`)
	repoDir := setupFactory(t, instruction)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)

	later := runTick(t, repoDir)
	err = runBuilder(t, repoDir, later)
	require.NoError(t, err)

	assert.Len(t, cursorRequests(t), 1)
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	assert.Len(t, works, 1)
	agents := listAgents(t, db)
	assert.Len(t, agents, 1)
}

func Test_RunBuilder_should_wait_when_the_builder_instruction_hash_differs(t *testing.T) {
	resetWireMock(t)
	repoDir := setupFactory(t, instruction)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	writeInstruction(t, repoDir, "builder", "A different instruction.")

	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [features] instruction changed")
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	assert.Empty(t, works)
	assert.Empty(t, cursorRequests(t))
}

func Test_RunBuilder_should_set_attention_when_the_create_is_refused(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubCursor(t, 500, `{"error":"nope"}`)
	stubLabels(t, 7)
	repoDir := setupFactory(t, instruction)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)

	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [7] rejected")
	db := openStateTip(t, repoDir)
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Equal(t, "attention", works[0].Status)
	agents := listAgents(t, db)
	require.Len(t, agents, 1)
	assert.False(t, agents[0].CursorRunID.Valid)
	assert.False(t, agents[0].FinishedAt.Valid)
	err = db.Close()
	require.NoError(t, err)
	labels := labelPosts(t)
	require.Len(t, labels, 1)
	assert.Equal(t, "/repos/octo/app/issues/7/labels", labels[0].url)
	assert.Contains(t, labels[0].body, "stop")

	later := runTick(t, repoDir)
	err = runBuilder(t, repoDir, later)
	require.NoError(t, err)

	assert.Len(t, cursorRequests(t), 1)
	assert.Len(t, labelPosts(t), 1)
	db = openStateTip(t, repoDir)
	defer db.Close()
	works = listWorks(t, db)
	assert.Len(t, works, 1)
	assert.Equal(t, "attention", works[0].Status)
	agents = listAgents(t, db)
	assert.Len(t, agents, 1)
}

func setupFactory(t *testing.T, text string) (repoDir string) {
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
	writeInstructions(t, repoDir, text)
	commitConfig(t, repoDir)
	useStateBranch(t, repoDir)
	return
}

func writeInstructions(t *testing.T, repoDir string, builder string) {
	t.Helper()
	writeInstruction(t, repoDir, "builder", builder)
	writeInstruction(t, repoDir, "standards_reviewer", "")
	writeInstruction(t, repoDir, "specification_reviewer", "")
	writeInstruction(t, repoDir, "fixer", "")
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

func assertAbsentTable(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var found string
	query := `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`
	err := db.QueryRowContext(t.Context(), query, name).Scan(&found)
	require.ErrorIs(t, err, sql.ErrNoRows)
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

func issueJSON(number int, title string, body string) (payload string) {
	payload = fmt.Sprintf(`[{"number":%d,"title":"%s","body":"%s","html_url":"https://github.com/octo/app/issues/%d","state":"open"}]`, number, title, body, number)
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
