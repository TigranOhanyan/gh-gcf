package specification

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/TigranOhanyan/gh-gcf/logging"
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
const envGHPermission = "GH_PERMISSION"
const envGHToken = "GH_TOKEN"
const envGitHubToken = "GITHUB_TOKEN"
const permissionWriteRead = "WRITE_READ"

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
		logging.From(ctx).Error("list works", zap.Any("owner", repo.Owner), zap.Any("name", repo.Name), zap.Error(err))
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
		logging.From(ctx).Error("development", zap.Any("development", work.DevelopmentName), zap.Error(err))
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
		logging.From(ctx).Info("instruction changed", zap.Any("development", development.Name))
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
		logging.From(ctx).Error("pull request", zap.Any("issue", work.IssueNumber), zap.Error(err))
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

	token, err := commands.InstallationToken(ctx, development.SpecificationReviewerApp, repo.Owner, repo.Name)
	if err != nil {
		reason := err.Error()
		line, err = rejectCreate(ctx, repo, stepID, client, development, work, agentID, reason)
		return
	}

	pullNumber := work.PullRequestNumber.Int64
	text := promptText(instruction, work.IssueNumber, work.IssueUrl, pullNumber)
	prompt := agent.CursorPrompt{Text: text}
	repoURL := fmt.Sprintf("https://github.com/%s/%s", repo.Owner, repo.Name)
	oneRepo := agent.CursorRepo{URL: repoURL}
	repos := []agent.CursorRepo{oneRepo}
	env := map[string]string{
		envGHPermission: permissionWriteRead,
		envGHToken:      token,
		envGitHubToken:  token,
	}
	body := agent.CursorCreate{
		Prompt:       prompt,
		AutoCreatePR: false,
		EnvVars:      env,
	}
	body.Repos = repos
	logging.From(ctx).Info("cursor create", zap.String("role", roleSpecification), zap.String("development", development.Name), zap.Int64("issue", work.IssueNumber))
	cursorAgentID, runID, raw, status, err := step.cursor.Create(ctx, body)
	lost := err != nil && status == 0
	if lost {
		return
	}
	refused := err != nil
	if refused {
		line, err = rejectCreate(ctx, repo, stepID, client, development, work, agentID, raw)
		return
	}

	err = ack(ctx, repo, stepID, agentID, cursorAgentID, runID)
	if err != nil {
		return
	}
	line = fmt.Sprintf("[%d] started", work.IssueNumber)
	return
}

func rejectCreate(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, development records.Development, work records.ListReadySpecificationWorksRow, agentID string, reason string) (line string, err error) {
	err = reject(ctx, repo, stepID, work.WorkID, agentID, reason)
	if err != nil {
		return
	}
	number := int(work.IssueNumber)
	err = commands.AddAttention(ctx, client, repo.Owner, repo.Name, number, development.AttentionLabel)
	if err != nil {
		return
	}
	logging.From(ctx).Info("attention", zap.Int("issue", number), zap.String("label", development.AttentionLabel), zap.String("reason", reason))
	line = fmt.Sprintf("[%d] rejected", work.IssueNumber)
	return
}

func claim(ctx context.Context, repo commands.Repo, stepID string, developmentName string, work records.ListReadySpecificationWorksRow, agentID string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		logging.From(ctx).Error("begin run", zap.Error(err))
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

func ack(ctx context.Context, repo commands.Repo, stepID string, agentID string, cursorAgentID string, runID string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		logging.From(ctx).Error("begin run", zap.Error(err))
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	storedAgent := sql.NullString{String: cursorAgentID, Valid: true}
	storedRun := sql.NullString{String: runID, Valid: true}
	runParams := records.SetDevelopmentAgentCursorIDsParams{
		CursorAgentID: storedAgent,
		CursorRunID:   storedRun,
		ID:            agentID,
	}
	err = queries.SetDevelopmentAgentCursorIDs(ctx, runParams)
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
		logging.From(ctx).Error("begin run", zap.Error(err))
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
	trailer := agent.ReviewerTrailer()
	text = instruction + "\n\n" + header + "\n\n" + issue + "\n\n" + trailer
	return
}

func newAgentID() (id string, err error) {
	value, err := uuid.NewV7()
	if err != nil {
		zap.L().Error("new agent id", zap.Error(err))
		return
	}
	id = "bc-" + value.String()
	return
}

func newEventID() (id string, err error) {
	timestamp := ulid.Timestamp(time.Now())
	value, err := ulid.New(timestamp, rand.Reader)
	if err != nil {
		zap.L().Error("new event id", zap.Error(err))
		return
	}
	id = value.String()
	return
}
