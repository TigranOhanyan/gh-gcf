package fixer

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"
	"github.com/oklog/ulid/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const stepName = "check_fixer"
const eventCompleted = "completed"
const eventFailed = "failed"
const statusCompleted = "completed"
const conclusionFailure = "failure"
const conclusionCancelled = "cancelled"
const conclusionTimedOut = "timed_out"

type checkStep struct {
	GitHub *github.Client
	app    commands.FactoryApp
	cursor cursorClient
}

func (checkStep) Name() (name string) {
	name = stepName
	return
}

func (step checkStep) Apply(ctx context.Context, repo commands.Repo, _ records.Tick, stepRow records.Step) (message string, err error) {
	finished, err := repo.State.Queries.ListWorksReadyForFixerChecks(ctx)
	if err != nil {
		log.Error("list fixer checks", "owner", repo.Owner, "name", repo.Name, "err", err)
		return
	}
	pending, err := repo.State.Queries.ListPendingDevelopmentFixers(ctx)
	if err != nil {
		log.Error("list fixers", "owner", repo.Owner, "name", repo.Name, "err", err)
		return
	}
	none := len(finished) == 0 && len(pending) == 0
	if none {
		return
	}

	client, err := commands.GitHubClient(ctx, step.app, repo.Owner, repo.Name, step.GitHub)
	if err != nil {
		return
	}

	var lines []string
	for _, work := range finished {
		var line string
		line, err = step.judgeChecks(ctx, repo, client, work)
		if line != "" {
			lines = append(lines, line)
		}
		failed := err != nil
		if failed {
			message = strings.Join(lines, "\n")
			detail := err.Error()
			err = fmt.Errorf("[%d] %s", work.IssueNumber, detail)
			return
		}
	}
	for _, item := range pending {
		var line string
		line, err = step.observe(ctx, repo, stepRow.ID, client, item)
		if line != "" {
			lines = append(lines, line)
		}
		failed := err != nil
		if failed {
			message = strings.Join(lines, "\n")
			detail := err.Error()
			err = fmt.Errorf("[%d] %s", item.IssueNumber, detail)
			return
		}
	}
	message = strings.Join(lines, "\n")
	return
}

func (step checkStep) judgeChecks(ctx context.Context, repo commands.Repo, client *github.Client, work records.ListWorksReadyForFixerChecksRow) (line string, err error) {
	names, err := repo.State.Queries.ListDevelopmentCheckNames(ctx, work.DevelopmentName)
	if err != nil {
		log.Error("list checks", "development", work.DevelopmentName, "err", err)
		return
	}
	listed := len(names) > 0
	if listed {
		sha := work.FixerHeadSha.String
		invalid := !work.FixerHeadSha.Valid
		empty := sha == ""
		missingHead := invalid || empty
		if missingHead {
			err = fmt.Errorf("fixer head is missing")
			log.Error("fixer head", "issue", work.IssueNumber, "err", err)
			return
		}
		var runs []*github.CheckRun
		runs, err = listCheckRuns(ctx, client, repo.Owner, repo.Name, sha)
		if err != nil {
			return
		}
		waiting := listedChecksWaiting(names, runs)
		if waiting {
			log.Info("checks waiting", "issue", work.IssueNumber)
			return
		}
		red := listedChecksRed(names, runs)
		if red {
			line, err = step.stopForChecks(ctx, repo, client, work)
			return
		}
	}
	line, err = step.finishWork(ctx, repo, work)
	return
}

func (step checkStep) stopForChecks(ctx context.Context, repo commands.Repo, client *github.Client, work records.ListWorksReadyForFixerChecksRow) (line string, err error) {
	development, err := repo.State.Queries.GetDevelopment(ctx, work.DevelopmentName)
	if err != nil {
		log.Error("development", "development", work.DevelopmentName, "err", err)
		return
	}
	err = markAttention(ctx, repo, work.WorkID)
	if err != nil {
		return
	}
	number := int(work.IssueNumber)
	err = commands.AddAttention(ctx, client, repo.Owner, repo.Name, number, development.AttentionLabel)
	if err != nil {
		return
	}
	line = fmt.Sprintf("[%d] check is red", work.IssueNumber)
	return
}

func (step checkStep) finishWork(ctx context.Context, repo commands.Repo, work records.ListWorksReadyForFixerChecksRow) (line string, err error) {
	err = markFinished(ctx, repo, work.WorkID)
	if err != nil {
		return
	}
	line = fmt.Sprintf("[%d] finished", work.IssueNumber)
	return
}

func markAttention(ctx context.Context, repo commands.Repo, workID string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin check", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	err = queries.SetDevelopmentWorkAttention(ctx, workID)
	err = commands.FinishTx(tx, err)
	return
}

func markFinished(ctx context.Context, repo commands.Repo, workID string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin check", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	err = queries.SetDevelopmentWorkFinished(ctx, workID)
	err = commands.FinishTx(tx, err)
	return
}

func (step checkStep) observe(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, item records.ListPendingDevelopmentFixersRow) (line string, err error) {
	cursorAgentID := item.CursorAgentID.String
	runID := item.CursorRunID.String
	run, _, err := step.cursor.getRun(ctx, cursorAgentID, runID)
	if err != nil {
		return
	}
	log.Info("cursor run", "agent", cursorAgentID, "status", run.Status)

	stillRunning := run.Status == runCreating || run.Status == runRunning
	if stillRunning {
		return
	}
	errored := run.Status == runError
	cancelled := run.Status == runCancelled
	expired := run.Status == runExpired
	failed := errored || cancelled || expired
	if failed {
		reason := fmt.Sprintf("[%d] %s", item.IssueNumber, run.Status)
		line, err = step.stopForPerson(ctx, repo, stepID, client, item, reason)
		return
	}
	finished := run.Status == runFinished
	if !finished {
		return
	}
	line, err = step.finishFixer(ctx, repo, stepID, client, item)
	return
}

func (step checkStep) finishFixer(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, item records.ListPendingDevelopmentFixersRow) (line string, err error) {
	missing := !item.PullRequestNumber.Valid
	if missing {
		line, err = step.missingPull(ctx, repo, stepID, client, item)
		return
	}
	number := int(item.PullRequestNumber.Int64)
	sha, err := headSHA(ctx, client, repo.Owner, repo.Name, number)
	if err != nil {
		return
	}
	line = fmt.Sprintf("[%d] fixed", item.IssueNumber)
	err = markFixed(ctx, repo, stepID, item.AgentID, item.WorkID, sha, line)
	if err != nil {
		line = ""
	}
	return
}

func (step checkStep) missingPull(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, item records.ListPendingDevelopmentFixersRow) (line string, err error) {
	reason := fmt.Sprintf("[%d] pull request is missing", item.IssueNumber)
	line, err = step.stopForPerson(ctx, repo, stepID, client, item, reason)
	return
}

func (step checkStep) stopForPerson(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, item records.ListPendingDevelopmentFixersRow, reason string) (line string, err error) {
	development, err := repo.State.Queries.GetDevelopment(ctx, item.DevelopmentName)
	if err != nil {
		log.Error("development", "development", item.DevelopmentName, "err", err)
		return
	}
	err = markFailed(ctx, repo, stepID, item.WorkID, item.AgentID, reason)
	if err != nil {
		return
	}
	number := int(item.IssueNumber)
	err = commands.AddAttention(ctx, client, repo.Owner, repo.Name, number, development.AttentionLabel)
	if err != nil {
		return
	}
	line = reason
	return
}

func markFixed(ctx context.Context, repo commands.Repo, stepID string, agentID string, workID string, sha string, message string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin check", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	head := sql.NullString{String: sha, Valid: true}
	params := records.SetDevelopmentWorkFixerHeadParams{
		FixerHeadSha: head,
		ID:           workID,
	}
	err = queries.SetDevelopmentWorkFixerHead(ctx, params)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	err = queries.SetDevelopmentAgentFinished(ctx, agentID)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	err = completedEvent(ctx, queries, stepID, agentID, message)
	err = commands.FinishTx(tx, err)
	return
}

func markFailed(ctx context.Context, repo commands.Repo, stepID string, workID string, agentID string, reason string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin check", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	eventID, err := newEventID()
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	params := records.CreateDevelopmentEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    eventFailed,
		Message: reason,
	}
	err = queries.CreateDevelopmentEvent(ctx, params)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	err = queries.SetDevelopmentWorkAttention(ctx, workID)
	err = commands.FinishTx(tx, err)
	return
}

func completedEvent(ctx context.Context, queries *records.Queries, stepID string, agentID string, message string) (err error) {
	eventID, err := newEventID()
	if err != nil {
		return
	}
	params := records.CreateDevelopmentEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    eventCompleted,
		Message: message,
	}
	err = queries.CreateDevelopmentEvent(ctx, params)
	if err != nil {
		log.Error("create event", "agent", agentID, "err", err)
	}
	return
}

func newEventID() (id string, err error) {
	timestamp := ulid.Timestamp(time.Now())
	value, err := ulid.New(timestamp, rand.Reader)
	if err != nil {
		log.Error("new event id", "err", err)
		return
	}
	id = value.String()
	return
}

func listedChecksWaiting(names []string, runs []*github.CheckRun) (waiting bool) {
	newest := newestListed(names, runs)
	for _, name := range names {
		run, seen := newest[name]
		missing := !seen
		if missing {
			waiting = true
			return
		}
		finished := run.GetStatus() == statusCompleted
		if !finished {
			waiting = true
			return
		}
	}
	return
}

func listedChecksRed(names []string, runs []*github.CheckRun) (red bool) {
	newest := newestListed(names, runs)
	for _, name := range names {
		run, seen := newest[name]
		missing := !seen
		if missing {
			continue
		}
		failed := redConclusion(run.GetConclusion())
		if failed {
			red = true
			return
		}
	}
	return
}

func newestListed(names []string, runs []*github.CheckRun) (newest map[string]*github.CheckRun) {
	listed := map[string]struct{}{}
	for _, name := range names {
		listed[name] = struct{}{}
	}
	newest = map[string]*github.CheckRun{}
	for _, run := range runs {
		name := run.GetName()
		_, known := listed[name]
		if !known {
			continue
		}
		kept, seen := newest[name]
		if !seen {
			newest[name] = run
			continue
		}
		later := newerRun(run, kept)
		if later {
			newest[name] = run
		}
	}
	return
}

func redConclusion(conclusion string) (red bool) {
	failure := conclusion == conclusionFailure
	cancelled := conclusion == conclusionCancelled
	timedOut := conclusion == conclusionTimedOut
	red = failure || cancelled || timedOut
	return
}

func newerRun(candidate *github.CheckRun, kept *github.CheckRun) (newer bool) {
	candidateAt := candidate.GetStartedAt()
	keptAt := kept.GetStartedAt()
	candidateMissing := candidateAt.IsZero()
	if candidateMissing {
		return
	}
	keptMissing := keptAt.IsZero()
	if keptMissing {
		newer = true
		return
	}
	newer = candidateAt.After(keptAt.Time)
	return
}
