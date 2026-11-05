package builder

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
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const stepName = "builder"
const roleBuilder = "builder"
const eventStarting = "starting"
const eventAck = "ack"
const eventRejected = "rejected"

type builderStep struct {
	GitHub *github.Client
	app    commands.FactoryApp
	cursor cursorClient
}

func (builderStep) Name() (name string) {
	name = stepName
	return
}

func (step builderStep) Apply(ctx context.Context, repo commands.Repo, _ records.Tick, stepRow records.Step) (message string, err error) {
	builders, err := repo.State.Queries.ListBuilderMembers(ctx)
	if err != nil {
		log.Error("list builders", "owner", repo.Owner, "name", repo.Name, "err", err)
		return
	}
	none := len(builders) == 0
	if none {
		return
	}

	client, err := commands.GitHubClient(ctx, step.app, repo.Owner, repo.Name, step.GitHub)
	if err != nil {
		return
	}

	var lines []string
	for _, member := range builders {
		added, callErr := step.startMember(ctx, repo, stepRow.ID, client, member)
		lines = append(lines, added...)
		failed := callErr != nil
		if failed {
			message = strings.Join(lines, "\n")
			err = fmt.Errorf("[%s] %s", member.Name, callErr.Error())
			return
		}
	}
	message = strings.Join(lines, "\n")
	return
}

func (step builderStep) startMember(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, member records.ListBuilderMembersRow) (lines []string, err error) {
	sum, err := commands.InstructionFingerprint(repo.Dir, member.Name)
	if err != nil {
		return
	}
	changed := sum != member.InstructionSha256
	if changed {
		line := fmt.Sprintf("[%s] instruction changed", member.Name)
		lines = append(lines, line)
		log.Info("instruction changed", "member", member.Name)
		return
	}

	text, err := commands.ReadInstruction(repo.Dir, member.Name)
	if err != nil {
		return
	}
	issues, err := listOpenIssues(ctx, client, repo.Owner, repo.Name, member.StartingLabel)
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
		line, err = step.startIssue(ctx, repo, stepID, client, member, text, issue)
		if line != "" {
			lines = append(lines, line)
		}
		if err != nil {
			return
		}
	}
	return
}

func (step builderStep) startIssue(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, member records.ListBuilderMembersRow, instruction string, issue *github.Issue) (line string, err error) {
	number := issue.GetNumber()
	_, err = repo.State.Queries.GetCodingWorkByIssue(ctx, int64(number))
	absent := errors.Is(err, sql.ErrNoRows)
	lookupFailed := err != nil && !absent
	if lookupFailed {
		log.Error("coding work", "issue", number, "err", err)
		return
	}
	err = nil
	if !absent {
		return
	}

	linked, err := referencedByPullRequest(ctx, client, repo.Owner, repo.Name, number)
	if err != nil {
		return
	}
	if linked {
		line = fmt.Sprintf("[%d] pull request exists", number)
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
	err = claim(ctx, repo, stepID, member, issue, agentID, workID)
	if err != nil {
		return
	}

	body := cursorCreate{
		Prompt:       cursorPrompt{Text: promptText(instruction, issue)},
		AutoCreatePR: true,
		AgentID:      agentID,
	}
	repoURL := fmt.Sprintf("https://github.com/%s/%s", repo.Owner, repo.Name)
	body.Repos = []cursorRepo{{URL: repoURL}}
	runID, raw, status, err := step.cursor.create(ctx, body)
	lost := err != nil && status == 0
	if lost {
		return
	}
	if err != nil {
		rejectErr := reject(ctx, repo, stepID, agentID, raw)
		if rejectErr != nil {
			err = rejectErr
			return
		}
		labelErr := addAttention(ctx, client, repo.Owner, repo.Name, number, member.AttentionLabel)
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

func claim(ctx context.Context, repo commands.Repo, stepID string, member records.ListBuilderMembersRow, issue *github.Issue, agentID string, workID string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin builder", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	number := issue.GetNumber()
	issueParams := records.CreateIssueParams{
		Number: int64(number),
		Url:    issueURL(repo, issue),
	}
	err = queries.CreateIssue(ctx, issueParams)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	workParams := records.CreateCodingWorkParams{
		ID:          workID,
		IssueNumber: int64(number),
		BuilderName: member.Name,
	}
	err = queries.CreateCodingWork(ctx, workParams)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	agentParams := records.CreateAgentParams{
		ID:         agentID,
		Role:       roleBuilder,
		WorkID:     workID,
		MemberName: member.Name,
	}
	err = queries.CreateAgent(ctx, agentParams)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	err = queries.CreateBuilderAgent(ctx, agentID)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	eventID, err := newEventID()
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	eventParams := records.CreateEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    eventStarting,
		Message: fmt.Sprintf("[%s] [%d] starting", member.Name, number),
	}
	err = queries.CreateEvent(ctx, eventParams)
	err = commands.FinishTx(tx, err)
	return
}

func ack(ctx context.Context, repo commands.Repo, stepID string, agentID string, runID string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin builder", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	run := sql.NullString{String: runID, Valid: true}
	runParams := records.SetAgentRunIDParams{
		CursorRunID: run,
		ID:          agentID,
	}
	err = queries.SetAgentRunID(ctx, runParams)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	eventID, err := newEventID()
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	eventParams := records.CreateEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    eventAck,
		Message: fmt.Sprintf("[%s] acknowledged", agentID),
	}
	err = queries.CreateEvent(ctx, eventParams)
	err = commands.FinishTx(tx, err)
	return
}

func reject(ctx context.Context, repo commands.Repo, stepID string, agentID string, reason string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin builder", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	eventID, err := newEventID()
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	eventParams := records.CreateEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    eventRejected,
		Message: reason,
	}
	err = queries.CreateEvent(ctx, eventParams)
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
		log.Error("new coding work id", "err", err)
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
