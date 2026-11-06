package coding_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wiremock/go-wiremock"

	"github.com/TigranOhanyan/gh-gcf/commands/coding"
	"github.com/TigranOhanyan/gh-gcf/commands/compile"
	"github.com/TigranOhanyan/gh-gcf/commands/tick"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const owner = "octo"
const repoName = "app"
const token = "test-token"
const instruction = "Build the issue."

func Test_Builder_should_start_an_agent_for_an_open_issue(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubTimeline(t, 7, `[]`)
	stubCursor(t, 200, `{"run":{"id":"run-11111111-1111-7111-8111-111111111111"}}`)
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
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Equal(t, int64(7), works[0].IssueNumber)
	assert.Equal(t, "features", works[0].BuilderName)
	agents := listAgents(t, db)
	require.Len(t, agents, 1)
	assert.True(t, strings.HasPrefix(agents[0].ID, "bc-"))
	assert.Equal(t, "builder", agents[0].Role)
	assert.Equal(t, works[0].ID, agents[0].WorkID)
	assert.True(t, agents[0].CursorRunID.Valid)
	assert.Equal(t, "run-11111111-1111-7111-8111-111111111111", agents[0].CursorRunID.String)
	built := listBuilderAgents(t, db)
	require.Len(t, built, 1)
	assert.Equal(t, agents[0].ID, built[0].ID)
	events := listEvents(t, db)
	require.Len(t, events, 2)
	assert.Equal(t, "starting", events[0].Kind)
	assert.Equal(t, agents[0].ID, events[0].AgentID)
	assert.Equal(t, "ack", events[1].Kind)
	assert.Equal(t, agents[0].ID, events[1].AgentID)
	requests := cursorRequests(t)
	require.Len(t, requests, 1)
	assert.Contains(t, requests[0], instruction)
	assert.Contains(t, requests[0], "Issue #7: Add a thing")
	assert.Contains(t, requests[0], "Do it")
	assert.Contains(t, requests[0], agents[0].ID)
}

func Test_Builder_should_leave_a_taken_issue(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubTimeline(t, 7, `[]`)
	stubCursor(t, 200, `{"run":{"id":"run-11111111-1111-7111-8111-111111111111"}}`)
	repoDir := setupFactory(t, instruction)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)

	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)

	requests := cursorRequests(t)
	assert.Len(t, requests, 1)
	db := openStateTip(t, repoDir)
	defer db.Close()
	agents := listAgents(t, db)
	assert.Len(t, agents, 1)
}

func Test_Builder_should_skip_an_issue_that_has_a_pull_request(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	timeline := `[{"event":"cross-referenced","source":{"issue":{"number":8,"pull_request":{"url":"https://api.github.com/repos/octo/app/pulls/8"}}}}]`
	stubTimeline(t, 7, timeline)
	repoDir := setupFactory(t, instruction)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)

	err = runBuilder(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [7] pull request exists")
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	assert.Empty(t, works)
	assert.Empty(t, cursorRequests(t))
}

func Test_Builder_should_skip_a_member_whose_instruction_changed(t *testing.T) {
	resetWireMock(t)
	repoDir := setupFactory(t, instruction)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	writeInstruction(t, repoDir, "features", "A different instruction.")

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

func Test_Builder_should_reject_a_failed_create(t *testing.T) {
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubTimeline(t, 7, `[]`)
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
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	agents := listAgents(t, db)
	require.Len(t, agents, 1)
	assert.False(t, agents[0].CursorRunID.Valid)
	events := listEvents(t, db)
	require.Len(t, events, 2)
	assert.Equal(t, "starting", events[0].Kind)
	assert.Equal(t, "rejected", events[1].Kind)
	assert.Equal(t, `{"error":"nope"}`, events[1].Message)
	assert.Len(t, labelRequests(t), 1)
}

func Test_Builder_should_require_a_tick_id(t *testing.T) {
	cmd := coding.NewCoding()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tick")
}

func setupFactory(t *testing.T, text string) (repoDir string) {
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
	err = configDB.Queries.CreateLabel(ctx, configrecords.CreateLabelParams{Name: "todo", Color: "ededed"})
	require.NoError(t, err)
	err = configDB.Queries.CreateLabel(ctx, configrecords.CreateLabelParams{Name: "attention", Color: "ededed"})
	require.NoError(t, err)
	err = configDB.Queries.CreateMember(ctx, configrecords.CreateMemberParams{
		Name:           "features",
		Role:           "builder",
		AttentionLabel: "attention",
	})
	require.NoError(t, err)
	err = configDB.Queries.CreateBuilderMember(ctx, configrecords.CreateBuilderMemberParams{
		Name:          "features",
		StartingLabel: "todo",
	})
	require.NoError(t, err)
	err = configDB.Close()
	require.NoError(t, err)
	writeInstruction(t, repoDir, "features", text)
	configRel := filepath.Join("repos", owner, repoName, "config.db")
	membersRel := filepath.Join("repos", owner, repoName, "members")
	addLine := fmt.Sprintf("add %s %s", configRel, membersRel)
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

func runCompile(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	cmd := compile.NewCompile()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	t.Chdir(repoDir)
	cmd.SetArgs([]string{"--tick", id})
	err = cmd.ExecuteContext(t.Context())
	return
}

func runBuilder(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Setenv("GH_TOKEN", token)
	cmd := coding.NewCoding()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	t.Chdir(repoDir)
	cmd.SetArgs([]string{"--tick", id})
	err = cmd.ExecuteContext(t.Context())
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

func listBuilderAgents(t *testing.T, db *statedb.DB) (rows []records.BuilderAgent) {
	t.Helper()
	rows, err := db.Queries.ListBuilderAgents(t.Context())
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

func labelRequests(t *testing.T) (bodies []string) {
	t.Helper()
	journal, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	for _, entry := range journal.Requests {
		request := entry.Request
		matched := request.Method == "POST" && strings.HasSuffix(request.URL, "/labels")
		if matched {
			bodies = append(bodies, request.Body)
		}
	}
	return
}
