package development

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/run"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const stepName = "run_development"
const roleBuilder = "builder"
const eventStarting = "starting"
const eventAck = "ack"
const eventRejected = "rejected"

type developmentStep struct {
	GitHub *github.Client
	app    commands.FactoryApp
	cursor run.CursorClient
}

func (developmentStep) Name() (name string) {
	name = stepName
	return
}

func (step developmentStep) Apply(ctx context.Context, repo commands.Repo, _ records.Tick, stepRow records.Step) (message string, err error) {
	developments, err := repo.State.Queries.ListDevelopments(ctx)
	if err != nil {
		log.Error("list developments", "owner", repo.Owner, "name", repo.Name, "err", err)
		return
	}
	none := len(developments) == 0
	if none {
		return
	}

	client, err := commands.GitHubClient(ctx, step.app, repo.Owner, repo.Name, step.GitHub)
	if err != nil {
		return
	}

	var lines []string
	for _, item := range developments {
		added, callErr := step.startDevelopment(ctx, repo, stepRow.ID, client, item)
		lines = append(lines, added...)
		failed := callErr != nil
		if failed {
			message = strings.Join(lines, "\n")
			err = fmt.Errorf("[%s] %s", item.Name, callErr.Error())
			return
		}
	}
	message = strings.Join(lines, "\n")
	return
}

func (step developmentStep) startDevelopment(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, item records.Development) (lines []string, err error) {
	sum, err := commands.DevelopmentInstructionFingerprint(repo.Dir, item.Name, roleBuilder)
	if err != nil {
		return
	}
	changed := sum != item.BuilderInstructionSha256
	if changed {
		line := fmt.Sprintf("[%s] instruction changed", item.Name)
		lines = append(lines, line)
		log.Info("instruction changed", "development", item.Name)
		return
	}

	text, err := commands.ReadDevelopmentInstruction(repo.Dir, item.Name, roleBuilder)
	if err != nil {
		return
	}
	issues, err := run.ListOpenIssues(ctx, client, repo.Owner, repo.Name, item.StartingLabel)
	if err != nil {
		return
	}
	for _, issue := range issues {
		if issue == nil {
			continue
		}
		pull := issue.IsPullRequest()
		if pull {
			continue
		}
		var line string
		line, err = step.startIssue(ctx, repo, stepID, client, item, text, issue)
		if line != "" {
			lines = append(lines, line)
		}
		if err != nil {
			return
		}
	}
	return
}

func (step developmentStep) startIssue(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, item records.Development, instruction string, issue *github.Issue) (line string, err error) {
	number := issue.GetNumber()
	_, err = repo.State.Queries.GetDevelopmentWorkByIssue(ctx, int64(number))
	absent := errors.Is(err, sql.ErrNoRows)
	lookupFailed := err != nil && !absent
	if lookupFailed {
		log.Error("development work", "issue", number, "err", err)
		return
	}
	err = nil
	taken := !absent
	if taken {
		return
	}

	agentID, err := newAgentID()
	if err != nil {
		return
	}
	workID, err := newWorkID()
	if err != nil {
		return
	}
	err = claim(ctx, repo, stepID, item, issue, agentID, workID)
	if err != nil {
		return
	}

	text := promptText(instruction, issue)
	prompt := run.CursorPrompt{Text: text}
	repoURL := fmt.Sprintf("https://github.com/%s/%s", repo.Owner, repo.Name)
	oneRepo := run.CursorRepo{URL: repoURL}
	repos := []run.CursorRepo{oneRepo}
	body := run.CursorCreate{
		Prompt:       prompt,
		AutoCreatePR: true,
		AgentID:      agentID,
	}
	body.Repos = repos
	runID, raw, status, err := step.cursor.Create(ctx, body)
	lost := err != nil && status == 0
	if lost {
		return
	}
	refused := err != nil
	if refused {
		rejectErr := reject(ctx, repo, stepID, workID, agentID, raw)
		if rejectErr != nil {
			err = rejectErr
			return
		}
		labelErr := commands.AddAttention(ctx, client, repo.Owner, repo.Name, number, item.AttentionLabel)
		if labelErr != nil {
			err = labelErr
			return
		}
		line = fmt.Sprintf("[%d] rejected", number)
		err = nil
		return
	}

	err = ack(ctx, repo, stepID, agentID, runID)
	if err != nil {
		return
	}
	line = fmt.Sprintf("[%d] started", number)
	return
}

func claim(ctx context.Context, repo commands.Repo, stepID string, item records.Development, issue *github.Issue, agentID string, workID string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin run", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	agentParams := records.CreateDevelopmentAgentParams{
		ID:   agentID,
		Role: roleBuilder,
	}
	err = queries.CreateDevelopmentAgent(ctx, agentParams)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	number := issue.GetNumber()
	agentLink := sql.NullString{String: agentID, Valid: true}
	workParams := records.CreateDevelopmentWorkParams{
		ID:              workID,
		DevelopmentName: item.Name,
		IssueNumber:     int64(number),
		IssueUrl:        issueURL(repo, issue),
		BuilderAgent:    agentLink,
	}
	err = queries.CreateDevelopmentWork(ctx, workParams)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	eventID, err := newEventID()
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	eventParams := records.CreateDevelopmentEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    eventStarting,
		Message: fmt.Sprintf("[%s] [%d] starting", item.Name, number),
	}
	err = queries.CreateDevelopmentEvent(ctx, eventParams)
	err = commands.FinishTx(tx, err)
	return
}

func ack(ctx context.Context, repo commands.Repo, stepID string, agentID string, runID string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin run", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	run := sql.NullString{String: runID, Valid: true}
	runParams := records.SetDevelopmentAgentRunIDParams{
		CursorRunID: run,
		ID:          agentID,
	}
	err = queries.SetDevelopmentAgentRunID(ctx, runParams)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	eventID, err := newEventID()
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	eventParams := records.CreateDevelopmentEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    eventAck,
		Message: fmt.Sprintf("[%s] acknowledged", agentID),
	}
	err = queries.CreateDevelopmentEvent(ctx, eventParams)
	err = commands.FinishTx(tx, err)
	return
}

func reject(ctx context.Context, repo commands.Repo, stepID string, workID string, agentID string, reason string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin run", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	eventID, err := newEventID()
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	eventParams := records.CreateDevelopmentEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    eventRejected,
		Message: reason,
	}
	err = queries.CreateDevelopmentEvent(ctx, eventParams)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	err = queries.SetDevelopmentWorkAttention(ctx, workID)
	err = commands.FinishTx(tx, err)
	return
}

func promptText(instruction string, issue *github.Issue) (text string) {
	number := issue.GetNumber()
	title := issue.GetTitle()
	body := issue.GetBody()
	header := fmt.Sprintf("Issue #%d: %s", number, title)
	text = instruction + "\n\n" + header + "\n\n" + body
	return
}

func issueURL(repo commands.Repo, issue *github.Issue) (url string) {
	url = issue.GetHTMLURL()
	missing := url == ""
	if missing {
		url = fmt.Sprintf("https://github.com/%s/%s/issues/%d", repo.Owner, repo.Name, issue.GetNumber())
	}
	return
}

func newAgentID() (id string, err error) {
	value, err := uuid.NewV7()
	if err != nil {
		log.Error("new agent id", "err", err)
		return
	}
	id = "bc-" + value.String()
	return
}

func newWorkID() (id string, err error) {
	timestamp := ulid.Timestamp(time.Now())
	value, err := ulid.New(timestamp, rand.Reader)
	if err != nil {
		log.Error("new development work id", "err", err)
		return
	}
	id = value.String()
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
