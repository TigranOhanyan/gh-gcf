package specification

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"
	"github.com/oklog/ulid/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const stepName = "check_specification_reviewer"
const eventCompleted = "completed"
const eventFailed = "failed"

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
	pending, err := repo.State.Queries.ListPendingDevelopmentSpecificationReviewers(ctx)
	if err != nil {
		log.Error("list specification reviewers", "owner", repo.Owner, "name", repo.Name, "err", err)
		return
	}
	none := len(pending) == 0
	if none {
		return
	}

	client, err := commands.GitHubClient(ctx, step.app, repo.Owner, repo.Name, step.GitHub)
	if err != nil {
		return
	}

	var lines []string
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

func (step checkStep) observe(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, item records.ListPendingDevelopmentSpecificationReviewersRow) (line string, err error) {
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
	line, err = step.finishReviewer(ctx, repo, stepID, item)
	return
}

func (step checkStep) finishReviewer(ctx context.Context, repo commands.Repo, stepID string, item records.ListPendingDevelopmentSpecificationReviewersRow) (line string, err error) {
	line = fmt.Sprintf("[%d] specification reviewed", item.IssueNumber)
	err = markReviewed(ctx, repo, stepID, item.AgentID, line)
	if err != nil {
		line = ""
	}
	return
}

func (step checkStep) stopForPerson(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, item records.ListPendingDevelopmentSpecificationReviewersRow, reason string) (line string, err error) {
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

func markReviewed(ctx context.Context, repo commands.Repo, stepID string, agentID string, message string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin check", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
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
