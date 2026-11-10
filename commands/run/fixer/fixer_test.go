package fixer_test

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
	"github.com/wiremock/go-wiremock/journal"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const owner = "octo"
const repoName = "app"
const token = "test-token"
const developmentName = "features"
const checkName = "ci / unit"
const otherCheckName = "ci / lint"
const builderInstruction = "Build the issue."
const standardsInstruction = "Review the standards."
const specificationInstruction = "Review the specification."
const fixerInstruction = "Apply the requested changes."
const runID = "run-11111111-1111-7111-8111-111111111111"
const headSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const olderStart = "2026-11-08T09:00:00Z"
const newerStart = "2026-11-08T10:00:00Z"
const standardsInstallation int64 = 420
const specificationInstallation int64 = 430
const standardsBotID int64 = 4201
const specificationBotID int64 = 4301
const standardsToken = "token-standards"
const specificationToken = "token-specification"

func Test_RunFixer_should_wait_when_a_listed_checks_newest_run_has_not_finished_and_write_nothing(t *testing.T) {
	repoDir := readyReviewers(t)
	run := oneRun(checkName, "in_progress", "", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(run))
	before := stateCounts(t, repoDir)
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Empty(t, strings.TrimSpace(string(body)))
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	work := works[0]
	agents := listAgents(t, db)
	require.Len(t, agents, before.agents)
	builderAgent, builderFound := agentByRole(agents, "builder")
	require.True(t, builderFound)
	standardsAgent, standardsFound := agentByRole(agents, "standards_reviewer")
	require.True(t, standardsFound)
	specificationAgent, specificationFound := agentByRole(agents, "specification_reviewer")
	require.True(t, specificationFound)
	_, fixerFound := agentByRole(agents, "fixer")
	assert.False(t, fixerFound)
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
	assert.Equal(t, standardsAgent.ID, work.StandardsReviewerAgent.String)
	assert.True(t, work.SpecificationReviewerAgent.Valid)
	assert.Equal(t, specificationAgent.ID, work.SpecificationReviewerAgent.String)
	assert.False(t, work.FixerAgent.Valid)
	assert.Empty(t, work.Status)
	events := listEvents(t, db)
	assert.Len(t, events, before.events)
	requests := checkRunRequests(t)
	require.Len(t, requests, 1)
	assert.Equal(t, "/repos/octo/app/commits/"+headSHA+"/check-runs", requests[0].path)
	assert.Equal(t, "all", requests[0].filter)
	assert.Empty(t, requests[0].checkName)
	assert.Zero(t, reviewRequestCount(t))
	assert.Empty(t, labelPosts(t))
}

func Test_RunFixer_should_wait_when_a_listed_check_name_has_no_run_and_write_nothing(t *testing.T) {
	repoDir := readyReviewers(t)
	stubCheckRuns(t, headSHA, checkRunsPayload())
	before := stateCounts(t, repoDir)
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	assertWroteNothing(t, repoDir, before)
	requests := checkRunRequests(t)
	require.Len(t, requests, 1)
	assert.Equal(t, "all", requests[0].filter)
}

func Test_RunFixer_should_ignore_an_older_passed_run_of_a_listed_name_when_a_newer_run_of_that_name_has_not_finished(t *testing.T) {
	repoDir := readyReviewers(t)
	older := oneRun(checkName, "completed", "success", olderStart)
	newer := oneRun(checkName, "in_progress", "", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(older, newer))
	before := stateCounts(t, repoDir)
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	assertWroteNothing(t, repoDir, before)
	requests := checkRunRequests(t)
	require.Len(t, requests, 1)
	assert.Equal(t, "all", requests[0].filter)
	assert.Empty(t, requests[0].checkName)
}

func Test_RunFixer_should_finish_the_work_when_an_unlisted_check_is_still_running_and_every_listed_check_passed(t *testing.T) {
	repoDir := readyReviewers(t)
	recordApps(t, repoDir)
	listed := oneRun(checkName, "completed", "success", olderStart)
	unlisted := oneRun(otherCheckName, "in_progress", "", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(listed, unlisted))
	stubReviewerAccess(t)
	standards := reviewJSON(1, standardsBotID, "APPROVED", newerStart)
	specification := reviewJSON(2, specificationBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(standards, specification))
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Equal(t, "finished", works[0].Status)
	assert.False(t, works[0].FixerAgent.Valid)
	requests := checkRunRequests(t)
	require.Len(t, requests, 1)
	assert.Equal(t, "/repos/octo/app/commits/"+headSHA+"/check-runs", requests[0].path)
	assert.Empty(t, requests[0].checkName)
	assert.Equal(t, 2, reviewRequestCount(t))
	assert.Empty(t, labelPosts(t))
}

func Test_RunFixer_should_start_the_fixer_when_both_verdicts_approve_and_a_listed_check_is_red(t *testing.T) {
	repoDir := readyReviewers(t)
	recordApps(t, repoDir)
	run := oneRun(checkName, "completed", "failure", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(run))
	stubReviewerAccess(t)
	standards := reviewJSON(1, standardsBotID, "APPROVED", olderStart)
	specification := reviewJSON(2, specificationBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(standards, specification))
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	assertFixerStarted(t, repoDir)
	requests := checkRunRequests(t)
	require.Len(t, requests, 1)
	assert.Equal(t, "/repos/octo/app/commits/"+headSHA+"/check-runs", requests[0].path)
	assert.Equal(t, 2, reviewRequestCount(t))
	assert.Empty(t, labelPosts(t))
}

func Test_RunFixer_should_set_attention_when_a_verdict_is_missing_and_not_start_the_fixer(t *testing.T) {
	repoDir := readyReviewers(t)
	recordApps(t, repoDir)
	run := oneRun(checkName, "completed", "success", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(run))
	stubLabels(t, 7)
	stubReviewerAccess(t)
	specification := reviewJSON(2, specificationBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(specification))
	before := stateCounts(t, repoDir)
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	assertMissingVerdict(t, repoDir, before)
	assert.Equal(t, 2, reviewRequestCount(t))
}

func Test_RunFixer_should_treat_a_commented_review_as_a_missing_verdict_and_not_start_the_fixer(t *testing.T) {
	repoDir := readyReviewers(t)
	recordApps(t, repoDir)
	run := oneRun(checkName, "completed", "success", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(run))
	stubLabels(t, 7)
	stubReviewerAccess(t)
	standards := reviewJSON(1, standardsBotID, "COMMENTED", newerStart)
	specification := reviewJSON(2, specificationBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(standards, specification))
	before := stateCounts(t, repoDir)
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	assertMissingVerdict(t, repoDir, before)
	assert.Equal(t, 2, reviewRequestCount(t))
}

func Test_RunFixer_should_set_attention_when_a_listed_check_is_red_and_a_verdict_is_missing_and_not_start_the_fixer(t *testing.T) {
	repoDir := readyReviewers(t)
	recordApps(t, repoDir)
	run := oneRun(checkName, "completed", "failure", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(run))
	stubLabels(t, 7)
	stubReviewerAccess(t)
	standards := reviewJSON(1, standardsBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(standards))
	before := stateCounts(t, repoDir)
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	assertMissingVerdict(t, repoDir, before)
	requests := checkRunRequests(t)
	require.Len(t, requests, 1)
	assert.Equal(t, 2, reviewRequestCount(t))
}

func Test_RunFixer_should_finish_the_work_when_both_verdicts_approve_and_the_development_lists_no_checks(t *testing.T) {
	repoDir := prepareReviewers(t, false)
	recordApps(t, repoDir)
	stubReviewerAccess(t)
	standards := reviewJSON(1, standardsBotID, "APPROVED", olderStart)
	specification := reviewJSON(2, specificationBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(standards, specification))
	before := stateCounts(t, repoDir)
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	assertFinishedWork(t, repoDir, before)
	assert.Empty(t, checkRunRequests(t))
	assert.Equal(t, 2, reviewRequestCount(t))
	assert.Empty(t, labelPosts(t))
}

func Test_RunFixer_should_set_the_development_work_to_finished_when_both_verdicts_approve_and_every_listed_check_passed(t *testing.T) {
	repoDir := readyReviewers(t)
	recordApps(t, repoDir)
	run := oneRun(checkName, "completed", "success", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(run))
	stubReviewerAccess(t)
	standards := reviewJSON(1, standardsBotID, "APPROVED", olderStart)
	specification := reviewJSON(2, specificationBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(standards, specification))
	before := stateCounts(t, repoDir)
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	assertFinishedWork(t, repoDir, before)
	requests := checkRunRequests(t)
	require.Len(t, requests, 1)
	assert.Equal(t, "/repos/octo/app/commits/"+headSHA+"/check-runs", requests[0].path)
	assert.Equal(t, 2, reviewRequestCount(t))
}

func Test_RunFixer_should_leave_the_fixer_unset_when_the_development_work_is_finished_that_way(t *testing.T) {
	repoDir := readyReviewers(t)
	recordApps(t, repoDir)
	run := oneRun(checkName, "completed", "success", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(run))
	stubReviewerAccess(t)
	standards := reviewJSON(1, standardsBotID, "APPROVED", olderStart)
	specification := reviewJSON(2, specificationBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(standards, specification))
	creates := len(cursorRequests(t))
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Equal(t, "finished", works[0].Status)
	assert.False(t, works[0].FixerAgent.Valid)
	assert.False(t, works[0].FixerHeadSha.Valid)
	agents := listAgents(t, db)
	_, fixerFound := agentByRole(agents, "fixer")
	assert.False(t, fixerFound)
	assert.Len(t, cursorRequests(t), creates)
}

func Test_RunFixer_should_not_merge_ready_or_close_the_pull_request_when_the_development_work_is_finished_that_way(t *testing.T) {
	repoDir := readyReviewers(t)
	recordApps(t, repoDir)
	run := oneRun(checkName, "completed", "success", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(run))
	stubReviewerAccess(t)
	standards := reviewJSON(1, standardsBotID, "APPROVED", olderStart)
	specification := reviewJSON(2, specificationBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(standards, specification))
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Equal(t, "finished", works[0].Status)
	assert.Empty(t, pullMutations(t))
}

func Test_RunFixer_should_start_the_fixer_when_one_verdict_requests_changes(t *testing.T) {
	repoDir := readyReviewers(t)
	recordApps(t, repoDir)
	run := oneRun(checkName, "completed", "success", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(run))
	stubReviewerAccess(t)
	standards := reviewJSON(1, standardsBotID, "CHANGES_REQUESTED", olderStart)
	specification := reviewJSON(2, specificationBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(standards, specification))
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [7] started")
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	work := works[0]
	agents := listAgents(t, db)
	require.Len(t, agents, 4)
	builderAgent, builderFound := agentByRole(agents, "builder")
	require.True(t, builderFound)
	standardsAgent, standardsFound := agentByRole(agents, "standards_reviewer")
	require.True(t, standardsFound)
	specificationAgent, specificationFound := agentByRole(agents, "specification_reviewer")
	require.True(t, specificationFound)
	fixerAgent, fixerFound := agentByRole(agents, "fixer")
	require.True(t, fixerFound)
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
	assert.Equal(t, standardsAgent.ID, work.StandardsReviewerAgent.String)
	assert.True(t, work.SpecificationReviewerAgent.Valid)
	assert.Equal(t, specificationAgent.ID, work.SpecificationReviewerAgent.String)
	assert.True(t, work.FixerAgent.Valid)
	assert.Equal(t, fixerAgent.ID, work.FixerAgent.String)
	assert.Empty(t, work.Status)
	assert.True(t, strings.HasPrefix(fixerAgent.ID, "bc-"))
	assert.Equal(t, "fixer", fixerAgent.Role)
	assert.True(t, fixerAgent.CursorRunID.Valid)
	assert.Equal(t, runID, fixerAgent.CursorRunID.String)
	assert.NotEmpty(t, fixerAgent.StartedAt)
	assert.False(t, fixerAgent.FinishedAt.Valid)
	requests := cursorRequests(t)
	require.Len(t, requests, 4)
	request, found := requestFor(requests, fixerAgent.ID)
	require.True(t, found)
	assert.Contains(t, request, fixerInstruction)
	assert.Contains(t, request, "Pull request #9")
	assert.Contains(t, request, "Issue #7: https://github.com/octo/app/issues/7")
	assert.Contains(t, request, `"autoCreatePR":false`)
	assert.Empty(t, labelPosts(t))
}

func Test_RunFixer_should_start_the_fixer_when_a_listed_check_is_red(t *testing.T) {
	repoDir := readyReviewers(t)
	recordApps(t, repoDir)
	run := oneRun(checkName, "completed", "failure", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(run))
	stubReviewerAccess(t)
	standards := reviewJSON(1, standardsBotID, "CHANGES_REQUESTED", olderStart)
	specification := reviewJSON(2, specificationBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(standards, specification))
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	assertFixerStarted(t, repoDir)
	assert.Equal(t, 2, reviewRequestCount(t))
	assert.Empty(t, labelPosts(t))
}

func Test_RunFixer_should_record_the_fixers_started_time_and_a_starting_event(t *testing.T) {
	repoDir := readyReviewers(t)
	recordApps(t, repoDir)
	run := oneRun(checkName, "completed", "success", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(run))
	stubReviewerAccess(t)
	standards := reviewJSON(1, standardsBotID, "CHANGES_REQUESTED", olderStart)
	specification := reviewJSON(2, specificationBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(standards, specification))
	id := runTick(t, repoDir)
	startOfTest := time.Now().Add(-time.Second)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)
	startOfCheck := time.Now().Add(time.Second)

	db := openStateTip(t, repoDir)
	defer db.Close()
	agents := listAgents(t, db)
	fixerAgent, found := agentByRole(agents, "fixer")
	require.True(t, found)
	started, err := time.Parse(time.RFC3339Nano, fixerAgent.StartedAt)
	require.NoError(t, err)
	assert.WithinRange(t, started, startOfTest, startOfCheck)
	events := listEvents(t, db)
	starting, found := eventFor(events, fixerAgent.ID, "starting")
	require.True(t, found)
	assert.Len(t, starting.ID, 26)
	assert.Len(t, starting.StepID, 26)
	assert.Equal(t, fixerAgent.ID, starting.AgentID)
	assert.Equal(t, "starting", starting.Kind)
	assert.Equal(t, "[features] [7] starting", starting.Message)
	recorded, err := time.Parse(time.RFC3339Nano, starting.CreatedAt)
	require.NoError(t, err)
	assert.WithinRange(t, recorded, startOfTest, startOfCheck)
}

func Test_RunFixer_should_not_start_a_second_fixer_when_one_has_already_started(t *testing.T) {
	repoDir := readyReviewers(t)
	recordApps(t, repoDir)
	run := oneRun(checkName, "completed", "success", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(run))
	stubReviewerAccess(t)
	standards := reviewJSON(1, standardsBotID, "CHANGES_REQUESTED", olderStart)
	specification := reviewJSON(2, specificationBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(standards, specification))
	id := runTick(t, repoDir)
	err := runFixer(t, repoDir, id)
	require.NoError(t, err)
	started := stateCounts(t, repoDir)
	posts := len(cursorRequests(t))
	later := runTick(t, repoDir)

	err = runFixer(t, repoDir, later)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Empty(t, strings.TrimSpace(string(body)))
	db := openStateTip(t, repoDir)
	defer db.Close()
	agents := listAgents(t, db)
	assert.Len(t, agents, started.agents)
	fixers := 0
	for _, agent := range agents {
		matched := agent.Role == "fixer"
		if matched {
			fixers++
		}
	}
	assert.Equal(t, 1, fixers)
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Empty(t, works[0].Status)
	assert.True(t, works[0].FixerAgent.Valid)
	assert.Len(t, cursorRequests(t), posts)
}

func Test_RunFixer_should_wait_when_the_fixer_instruction_hash_differs(t *testing.T) {
	repoDir := readyReviewers(t)
	writeInstruction(t, repoDir, "fixer", "A different instruction.")
	recordApps(t, repoDir)
	run := oneRun(checkName, "completed", "success", newerStart)
	stubCheckRuns(t, headSHA, checkRunsPayload(run))
	stubReviewerAccess(t)
	standards := reviewJSON(1, standardsBotID, "CHANGES_REQUESTED", olderStart)
	specification := reviewJSON(2, specificationBotID, "APPROVED", newerStart)
	stubReviews(t, reviewsPayload(standards, specification))
	before := stateCounts(t, repoDir)
	posts := len(cursorRequests(t))
	id := runTick(t, repoDir)

	err := runFixer(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [features] instruction changed")
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Empty(t, works[0].Status)
	assert.False(t, works[0].FixerAgent.Valid)
	agents := listAgents(t, db)
	assert.Len(t, agents, before.agents)
	_, fixerFound := agentByRole(agents, "fixer")
	assert.False(t, fixerFound)
	assert.Len(t, cursorRequests(t), posts)
}

type counts struct {
	agents int
	events int
}

func stateCounts(t *testing.T, repoDir string) (counted counts) {
	t.Helper()
	db := openStateTip(t, repoDir)
	counted.agents = len(listAgents(t, db))
	counted.events = len(listEvents(t, db))
	err := db.Close()
	require.NoError(t, err)
	return
}

func assertFinishedWork(t *testing.T, repoDir string, before counts) {
	t.Helper()
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [7] finished")
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	work := works[0]
	agents := listAgents(t, db)
	require.Len(t, agents, before.agents)
	builderAgent, builderFound := agentByRole(agents, "builder")
	require.True(t, builderFound)
	standardsAgent, standardsFound := agentByRole(agents, "standards_reviewer")
	require.True(t, standardsFound)
	specificationAgent, specificationFound := agentByRole(agents, "specification_reviewer")
	require.True(t, specificationFound)
	_, fixerFound := agentByRole(agents, "fixer")
	assert.False(t, fixerFound)
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
	assert.Equal(t, standardsAgent.ID, work.StandardsReviewerAgent.String)
	assert.True(t, work.SpecificationReviewerAgent.Valid)
	assert.Equal(t, specificationAgent.ID, work.SpecificationReviewerAgent.String)
	assert.False(t, work.FixerAgent.Valid)
	assert.Equal(t, "finished", work.Status)
	events := listEvents(t, db)
	assert.Len(t, events, before.events)
}

func assertWroteNothing(t *testing.T, repoDir string, before counts) {
	t.Helper()
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Empty(t, strings.TrimSpace(string(body)))
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	assert.Empty(t, works[0].Status)
	assert.False(t, works[0].FixerAgent.Valid)
	assert.False(t, works[0].FixerHeadSha.Valid)
	agents := listAgents(t, db)
	assert.Len(t, agents, before.agents)
	_, fixerFound := agentByRole(agents, "fixer")
	assert.False(t, fixerFound)
	events := listEvents(t, db)
	assert.Len(t, events, before.events)
}

func readyReviewers(t *testing.T) (repoDir string) {
	t.Helper()
	repoDir = prepareReviewers(t, true)
	return
}

func prepareReviewers(t *testing.T, listed bool) (repoDir string) {
	t.Helper()
	resetWireMock(t)
	stubIssues(t, issueJSON(7, "Add a thing", "Do it"))
	stubCursor(t, 200, `{"run":{"id":"`+runID+`"}}`)
	repoDir = setupFactory(t, listed)
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
	standardsTick := runTick(t, repoDir)
	err = runStandards(t, repoDir, standardsTick)
	require.NoError(t, err)
	finishReviewer(t, repoDir, "standards_reviewer")
	specificationTick := runTick(t, repoDir)
	err = runSpecification(t, repoDir, specificationTick)
	require.NoError(t, err)
	finishReviewer(t, repoDir, "specification_reviewer")
	return
}

func setupFactory(t *testing.T, listed bool) (repoDir string) {
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
	if listed {
		checkLine := fmt.Sprintf("setup development check add --owner octo --repo app --development %s --name %q", developmentName, checkName)
		mustRoot(t, checkLine)
	}
	writeInstruction(t, repoDir, "builder", builderInstruction)
	writeInstruction(t, repoDir, "standards_reviewer", standardsInstruction)
	writeInstruction(t, repoDir, "specification_reviewer", specificationInstruction)
	writeInstruction(t, repoDir, "fixer", fixerInstruction)
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

func runSpecification(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Setenv("GH_TOKEN", token)
	t.Chdir(repoDir)
	line := fmt.Sprintf("run specification-reviewer --tick %s", id)
	_, err = runRoot(t, line)
	return
}

func runFixer(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Setenv("GH_TOKEN", token)
	t.Chdir(repoDir)
	line := fmt.Sprintf("run fixer --tick %s", id)
	_, err = runRoot(t, line)
	return
}

func finishReviewer(t *testing.T, repoDir string, role string) {
	t.Helper()
	db := openStateTip(t, repoDir)
	agents := listAgents(t, db)
	err := db.Close()
	require.NoError(t, err)
	reviewer, found := agentByRole(agents, role)
	require.True(t, found)
	stubRun(t, reviewer.ID, finishedRun("https://github.com/octo/app/pull/9"))
	id := runTick(t, repoDir)
	standards := role == "standards_reviewer"
	if standards {
		err = runCheckStandards(t, repoDir, id)
		require.NoError(t, err)
		return
	}
	err = runCheckSpecification(t, repoDir, id)
	require.NoError(t, err)
}

func runCheckStandards(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Setenv("GH_TOKEN", token)
	t.Chdir(repoDir)
	line := fmt.Sprintf("check standards-reviewer --tick %s", id)
	_, err = runRoot(t, line)
	return
}

func runCheckSpecification(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Setenv("GH_TOKEN", token)
	t.Chdir(repoDir)
	line := fmt.Sprintf("check specification-reviewer --tick %s", id)
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

func assertFixerStarted(t *testing.T, repoDir string) {
	t.Helper()
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [7] started")
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	work := works[0]
	assert.Empty(t, work.Status)
	assert.False(t, work.FixerHeadSha.Valid)
	assert.True(t, work.FixerAgent.Valid)
	agents := listAgents(t, db)
	fixerAgent, fixerFound := agentByRole(agents, "fixer")
	require.True(t, fixerFound)
	assert.Equal(t, fixerAgent.ID, work.FixerAgent.String)
	assert.Equal(t, "fixer", fixerAgent.Role)
	assert.True(t, fixerAgent.CursorRunID.Valid)
	assert.Equal(t, runID, fixerAgent.CursorRunID.String)
	assert.False(t, fixerAgent.FinishedAt.Valid)
	requests := cursorRequests(t)
	_, found := requestFor(requests, fixerAgent.ID)
	assert.True(t, found)
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

func issueJSON(number int, title string, body string) (payload string) {
	payload = fmt.Sprintf(`[{"number":%d,"title":"%s","body":"%s","html_url":"https://github.com/octo/app/issues/%d","state":"open"}]`, number, title, body, number)
	return
}

func finishedRun(prURL string) (payload string) {
	payload = fmt.Sprintf(`{"id":"%s","status":"FINISHED","git":{"branches":[{"repoUrl":"https://github.com/octo/app","prUrl":"%s"}]}}`, runID, prURL)
	return
}

func oneRun(name string, status string, conclusion string, started string) (payload string) {
	bare := conclusion == ""
	if bare {
		payload = fmt.Sprintf(`{"name":%q,"status":%q,"started_at":%q}`, name, status, started)
		return
	}
	payload = fmt.Sprintf(`{"name":%q,"status":%q,"conclusion":%q,"started_at":%q}`, name, status, conclusion, started)
	return
}

func checkRunsPayload(runs ...string) (payload string) {
	joined := strings.Join(runs, ",")
	payload = fmt.Sprintf(`{"total_count":%d,"check_runs":[%s]}`, len(runs), joined)
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

func stubCheckRuns(t *testing.T, sha string, payload string) {
	t.Helper()
	path := fmt.Sprintf("/repos/octo/app/commits/%s/check-runs", sha)
	rule := wiremock.Get(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(payload),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

type checkRunRequest struct {
	path      string
	filter    string
	checkName string
}

func checkRunRequests(t *testing.T) (requests []checkRunRequest) {
	t.Helper()
	journal, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	for _, entry := range journal.Requests {
		request := entry.Request
		matched := request.Method == "GET" && strings.Contains(request.URL, "/check-runs")
		if matched {
			path, _, _ := strings.Cut(request.URL, "?")
			item := checkRunRequest{
				path:      path,
				filter:    queryValue(request.URL, request.QueryParams, "filter"),
				checkName: queryValue(request.URL, request.QueryParams, "check_name"),
			}
			requests = append(requests, item)
		}
	}
	return
}

func queryValue(raw string, params journal.Params, name string) (value string) {
	param, found := params[name]
	hasValue := found && len(param.Values) > 0
	if hasValue {
		value = param.Values[0]
		return
	}
	_, query, present := strings.Cut(raw, "?")
	if !present {
		return
	}
	for _, pair := range strings.Split(query, "&") {
		key, item, ok := strings.Cut(pair, "=")
		matched := ok && key == name
		if matched {
			value = item
			return
		}
	}
	return
}

func reviewRequestCount(t *testing.T) (count int) {
	t.Helper()
	journal, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	for _, entry := range journal.Requests {
		request := entry.Request
		matched := strings.Contains(request.URL, "reviews")
		if matched {
			count++
		}
	}
	return
}

type labelPost struct {
	url  string
	body string
}

func assertMissingVerdict(t *testing.T, repoDir string, before counts) {
	t.Helper()
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [7] verdict is missing")
	db := openStateTip(t, repoDir)
	defer db.Close()
	works := listWorks(t, db)
	require.Len(t, works, 1)
	work := works[0]
	agents := listAgents(t, db)
	require.Len(t, agents, before.agents)
	builderAgent, builderFound := agentByRole(agents, "builder")
	require.True(t, builderFound)
	standardsAgent, standardsFound := agentByRole(agents, "standards_reviewer")
	require.True(t, standardsFound)
	specificationAgent, specificationFound := agentByRole(agents, "specification_reviewer")
	require.True(t, specificationFound)
	_, fixerFound := agentByRole(agents, "fixer")
	assert.False(t, fixerFound)
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
	assert.Equal(t, standardsAgent.ID, work.StandardsReviewerAgent.String)
	assert.True(t, work.SpecificationReviewerAgent.Valid)
	assert.Equal(t, specificationAgent.ID, work.SpecificationReviewerAgent.String)
	assert.False(t, work.FixerAgent.Valid)
	assert.Equal(t, "attention", work.Status)
	events := listEvents(t, db)
	assert.Len(t, events, before.events)
	labels := labelPosts(t)
	require.Len(t, labels, 1)
	assert.Equal(t, "/repos/octo/app/issues/7/labels", labels[0].url)
	assert.Contains(t, labels[0].body, "stop")
}

func recordApps(t *testing.T, repoDir string) {
	t.Helper()
	stubInstallations(t)
	pemText := testAppKey(t)
	t.Setenv("FACTORY_GITHUB_APP_42", pemText)
	t.Setenv("FACTORY_GITHUB_APP_43", pemText)
	t.Setenv("FACTORY_GITHUB_APP_44", pemText)
	id := runTick(t, repoDir)
	err := runApp(t, repoDir, id)
	require.NoError(t, err)
}

func runApp(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Chdir(repoDir)
	line := fmt.Sprintf("compile app --tick %s", id)
	_, err = runRoot(t, line)
	return
}

func stubInstallations(t *testing.T) {
	t.Helper()
	path := "/repos/octo/app/installation"
	firstBody := fmt.Sprintf(`{"id":%d,"client_id":"Iv1.standards","suspended_at":null}`, standardsInstallation)
	secondBody := fmt.Sprintf(`{"id":%d,"client_id":"Iv1.specification","suspended_at":null}`, specificationInstallation)
	first := wiremock.Get(wiremock.URLPathEqualTo(path)).
		InScenario("installations").
		WhenScenarioStateIs(wiremock.ScenarioStateStarted).
		WillSetStateTo("second").
		WillReturnResponse(wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(firstBody))
	second := wiremock.Get(wiremock.URLPathEqualTo(path)).
		InScenario("installations").
		WhenScenarioStateIs("second").
		WillReturnResponse(wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(secondBody))
	err := wiremockClient.StubFor(first)
	require.NoError(t, err)
	err = wiremockClient.StubFor(second)
	require.NoError(t, err)
}

func stubReviewerAccess(t *testing.T) {
	t.Helper()
	stubInstallationToken(t, standardsInstallation, standardsToken)
	stubInstallationToken(t, specificationInstallation, specificationToken)
	stubUser(t, standardsToken, standardsBotID)
	stubUser(t, specificationToken, specificationBotID)
}

func stubInstallationToken(t *testing.T, installationID int64, token string) {
	t.Helper()
	path := fmt.Sprintf("/app/installations/%d/access_tokens", installationID)
	body := fmt.Sprintf(`{"token":%q}`, token)
	rule := wiremock.Post(wiremock.URLPathEqualTo(path)).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(body),
	)
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubUser(t *testing.T, token string, id int64) {
	t.Helper()
	body := fmt.Sprintf(`{"id":%d}`, id)
	rule := wiremock.Get(wiremock.URLPathEqualTo("/user")).
		WithHeader("Authorization", wiremock.EqualTo("Bearer "+token)).
		WillReturnResponse(wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(body))
	err := wiremockClient.StubFor(rule)
	require.NoError(t, err)
}

func stubReviews(t *testing.T, payload string) {
	t.Helper()
	rule := wiremock.Get(wiremock.URLPathEqualTo("/repos/octo/app/pulls/9/reviews")).WillReturnResponse(
		wiremock.NewResponse().WithStatus(200).WithHeader("Content-Type", "application/json").WithBody(payload),
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

func reviewJSON(id int64, userID int64, state string, submitted string) (payload string) {
	payload = fmt.Sprintf(`{"id":%d,"user":{"id":%d},"state":%q,"submitted_at":%q}`, id, userID, state, submitted)
	return
}

func reviewsPayload(reviews ...string) (payload string) {
	joined := strings.Join(reviews, ",")
	payload = fmt.Sprintf("[%s]", joined)
	return
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

func pullMutations(t *testing.T) (urls []string) {
	t.Helper()
	journal, err := wiremockClient.GetAllRequests()
	require.NoError(t, err)
	for _, entry := range journal.Requests {
		request := entry.Request
		merge := strings.Contains(request.URL, "/merge")
		write := request.Method == "PATCH" || request.Method == "PUT" || request.Method == "POST"
		pull := strings.Contains(request.URL, "/pulls/")
		issueWrite := (request.Method == "PATCH" || request.Method == "PUT") && strings.Contains(request.URL, "/issues/")
		matched := merge || (write && pull) || issueWrite
		if matched {
			urls = append(urls, request.Method+" "+request.URL)
		}
	}
	return
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
