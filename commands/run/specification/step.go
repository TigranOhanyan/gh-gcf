package specification

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/run/agent"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const stepName = "run_specification_reviewer"
const roleSpecification = "specification_reviewer"
const eventStarting = "starting"
const eventAck = "ack"
const eventRejected = "rejected"

type specificationStep struct {
	GitHub *github.Client
	app    commands.FactoryApp
	cursor agent.CursorClient
}

func (specificationStep) Name() (name string) {
	name = stepName
	return
}

func (step specificationStep) Apply(ctx context.Context, repo commands.Repo, _ records.Tick, stepRow records.Step) (message string, err error) {
	works, err := repo.State.Queries.ListReadySpecificationWorks(ctx)
	if err != nil {
		log.Error("list works", "owner", repo.Owner, "name", repo.Name, "err", err)
		return
	}
	none := len(works) == 0
	if none {
		return
	}

	client, err := commands.GitHubClient(ctx, step.app, repo.Owner, repo.Name, step.GitHub)
	if err != nil {
		return
	}

	noted := map[string]struct{}{}
	var lines []string
	for _, work := range works {
		var line string
		line, err = step.consider(ctx, repo, stepRow.ID, client, work, noted)
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
	message = strings.Join(lines, "\n")
	return
}

func (step specificationStep) consider(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, work records.ListReadySpecificationWorksRow, noted map[string]struct{}) (line string, err error) {
	development, err := repo.State.Queries.GetDevelopment(ctx, work.DevelopmentName)
	if err != nil {
		log.Error("development", "development", work.DevelopmentName, "err", err)
		return
	}
	sum, err := commands.DevelopmentInstructionFingerprint(repo.Dir, development.Name, roleSpecification)
	if err != nil {
		return
	}
	changed := sum != development.SpecificationReviewerInstructionSha256
	if changed {
		_, seen := noted[development.Name]
		if seen {
			return
		}
		noted[development.Name] = struct{}{}
		line = fmt.Sprintf("[%s] instruction changed", development.Name)
		log.Info("instruction changed", "development", development.Name)
		return
	}

	text, err := commands.ReadDevelopmentInstruction(repo.Dir, development.Name, roleSpecification)
	if err != nil {
		return
	}
	line, err = step.startWork(ctx, repo, stepID, client, development, work, text)
	return
}

func (step specificationStep) startWork(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, development records.Development, work records.ListReadySpecificationWorksRow, instruction string) (line string, err error) {
	missingPull := !work.PullRequestNumber.Valid
	if missingPull {
		err = fmt.Errorf("pull request is missing")
		log.Error("pull request", "issue", work.IssueNumber, "err", err)
		return
	}
	agentID, err := newAgentID()
	if err != nil {
		return
	}
	err = claim(ctx, repo, stepID, development.Name, work, agentID)
	if err != nil {
		return
	}

	pullNumber := work.PullRequestNumber.Int64
	text := promptText(instruction, work.IssueNumber, work.IssueUrl, pullNumber)
	prompt := agent.CursorPrompt{Text: text}
	repoURL := fmt.Sprintf("https://github.com/%s/%s", repo.Owner, repo.Name)
	oneRepo := agent.CursorRepo{URL: repoURL}
	repos := []agent.CursorRepo{oneRepo}
	body := agent.CursorCreate{
		Prompt:       prompt,
		AutoCreatePR: false,
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
		rejectErr := reject(ctx, repo, stepID, work.WorkID, agentID, raw)
		if rejectErr != nil {
			err = rejectErr
			return
		}
		number := int(work.IssueNumber)
		labelErr := commands.AddAttention(ctx, client, repo.Owner, repo.Name, number, development.AttentionLabel)
		if labelErr != nil {
			err = labelErr
			return
		}
		line = fmt.Sprintf("[%d] rejected", work.IssueNumber)
		err = nil
		return
	}

	err = ack(ctx, repo, stepID, agentID, runID)
	if err != nil {
		return
	}
	line = fmt.Sprintf("[%d] started", work.IssueNumber)
	return
}

func claim(ctx context.Context, repo commands.Repo, stepID string, developmentName string, work records.ListReadySpecificationWorksRow, agentID string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin run", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	agentParams := records.CreateDevelopmentAgentParams{
		ID:   agentID,
		Role: roleSpecification,
	}
	err = queries.CreateDevelopmentAgent(ctx, agentParams)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	link := sql.NullString{String: agentID, Valid: true}
	linkParams := records.SetSpecificationReviewerAgentParams{
		SpecificationReviewerAgent: link,
		ID:                         work.WorkID,
	}
	err = queries.SetSpecificationReviewerAgent(ctx, linkParams)
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
		Message: fmt.Sprintf("[%s] [%d] starting", developmentName, work.IssueNumber),
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
	stored := sql.NullString{String: runID, Valid: true}
	runParams := records.SetDevelopmentAgentRunIDParams{
		CursorRunID: stored,
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

func promptText(instruction string, issueNumber int64, issueURL string, pullNumber int64) (text string) {
	header := fmt.Sprintf("Pull request #%d", pullNumber)
	issue := fmt.Sprintf("Issue #%d: %s", issueNumber, issueURL)
	text = instruction + "\n\n" + header + "\n\n" + issue
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
