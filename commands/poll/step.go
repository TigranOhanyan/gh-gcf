package poll

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

const stepName = "poll_agents"
const roleBuilder = "builder"
const roleReviewer = "reviewer"
const roleFixer = "fixer"
const eventCompleted = "completed"
const eventFailed = "failed"

type pollStep struct {
	GitHub *github.Client
	app    commands.FactoryApp
	cursor cursorClient
}

func (pollStep) Name() (name string) {
	name = stepName
	return
}

func (step pollStep) Apply(ctx context.Context, repo commands.Repo, _ records.Tick, stepRow records.Step) (message string, err error) {
	pending, err := repo.State.Queries.ListPendingAgents(ctx)
	if err != nil {
		log.Error("list agents", "owner", repo.Owner, "name", repo.Name, "err", err)
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
	for _, agent := range pending {
		line, callErr := step.observe(ctx, repo, stepRow.ID, client, agent)
		if line != "" {
			lines = append(lines, line)
		}
		failed := callErr != nil
		if failed {
			message = strings.Join(lines, "\n")
			err = fmt.Errorf("[%s] %s", agent.MemberName, callErr.Error())
			return
		}
	}
	message = strings.Join(lines, "\n")
	return
}

func (step pollStep) observe(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, agent records.Agent) (line string, err error) {
	runID := agent.CursorRunID.String
	run, _, err := step.cursor.getRun(ctx, agent.ID, runID)
	if err != nil {
		return
	}
	log.Info("cursor run", "agent", agent.ID, "role", agent.Role, "status", run.Status)

	stillRunning := run.Status == runCreating || run.Status == runRunning
	if stillRunning {
		return
	}
	terminal := run.Status == runError || run.Status == runCancelled || run.Status == runExpired
	if terminal {
		reason := fmt.Sprintf("[%s] %s", agent.MemberName, run.Status)
		line, err = failAgent(ctx, repo, stepID, agent.ID, reason)
		return
	}
	finished := run.Status == runFinished
	if !finished {
		return
	}
	switch agent.Role {
	case roleBuilder:
		line, err = step.finishBuilder(ctx, repo, stepID, client, agent, run)
	case roleReviewer:
		line, err = step.finishReviewer(ctx, repo, stepID, agent)
	case roleFixer:
		line, err = step.finishFixer(ctx, repo, stepID, client, agent)
	default:
		err = fmt.Errorf("unknown role %s", agent.Role)
	}
	return
}

func (step pollStep) finishBuilder(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, agent records.Agent, run cursorRun) (line string, err error) {
	work, err := repo.State.Queries.GetCodingWork(ctx, agent.WorkID)
	if err != nil {
		log.Error("coding work", "work", agent.WorkID, "err", err)
		return
	}
	number, found := pullNumber(run.Git.Branches, repo.Owner, repo.Name)
	if !found {
		reason := fmt.Sprintf("[%d] pull request is missing", work.IssueNumber)
		line, err = failAgent(ctx, repo, stepID, agent.ID, reason)
		return
	}
	sha, err := headSHA(ctx, client, repo.Owner, repo.Name, number)
	if err != nil {
		return
	}
	line = fmt.Sprintf("[%d] built", work.IssueNumber)
	err = markBuilt(ctx, repo, stepID, agent.ID, work.ID, int64(number), sha, line)
	if err != nil {
		line = ""
	}
	return
}

func (step pollStep) finishReviewer(ctx context.Context, repo commands.Repo, stepID string, agent records.Agent) (line string, err error) {
	work, err := repo.State.Queries.GetCodingWork(ctx, agent.WorkID)
	if err != nil {
		log.Error("coding work", "work", agent.WorkID, "err", err)
		return
	}
	hasPull := work.PullRequestNumber.Valid
	if !hasPull {
		reason := fmt.Sprintf("[%d] pull request is missing", work.IssueNumber)
		line, err = failAgent(ctx, repo, stepID, agent.ID, reason)
		return
	}
	member, err := repo.State.Queries.GetReviewerMember(ctx, agent.MemberName)
	if err != nil {
		log.Error("reviewer", "member", agent.MemberName, "err", err)
		return
	}
	app, err := repo.State.Queries.GetGithubApp(ctx, member.GithubApp)
	if err != nil {
		log.Error("github app", "app", member.GithubApp, "err", err)
		return
	}
	number := int(work.PullRequestNumber.Int64)
	verdict, found, err := reviewerVerdict(ctx, repo.Owner, repo.Name, number, app.AppID, app.InstallationID)
	if err != nil {
		return
	}
	if !found {
		reason := fmt.Sprintf("[%s] verdict is missing", agent.MemberName)
		line, err = failAgent(ctx, repo, stepID, agent.ID, reason)
		return
	}
	line = fmt.Sprintf("[%s] %s", agent.MemberName, verdict)
	err = markVerdict(ctx, repo, stepID, agent.ID, verdict, line)
	if err != nil {
		line = ""
	}
	return
}

func (step pollStep) finishFixer(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, agent records.Agent) (line string, err error) {
	work, err := repo.State.Queries.GetCodingWork(ctx, agent.WorkID)
	if err != nil {
		log.Error("coding work", "work", agent.WorkID, "err", err)
		return
	}
	hasPull := work.PullRequestNumber.Valid
	if !hasPull {
		reason := fmt.Sprintf("[%d] pull request is missing", work.IssueNumber)
		line, err = failAgent(ctx, repo, stepID, agent.ID, reason)
		return
	}
	number := int(work.PullRequestNumber.Int64)
	sha, err := headSHA(ctx, client, repo.Owner, repo.Name, number)
	if err != nil {
		return
	}
	line = fmt.Sprintf("[%d] fixed", work.IssueNumber)
	err = markFixed(ctx, repo, stepID, agent.ID, work.ID, sha, line)
	if err != nil {
		line = ""
	}
	return
}

func markBuilt(ctx context.Context, repo commands.Repo, stepID string, agentID string, workID string, number int64, sha string, message string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin poll", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	pull := sql.NullInt64{Int64: number, Valid: true}
	head := sql.NullString{String: sha, Valid: true}
	params := records.MarkWorkBuiltParams{
		PullRequestNumber: pull,
		BuilderHeadSha:    head,
		ID:                workID,
	}
	err = queries.MarkWorkBuilt(ctx, params)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	err = completedEvent(ctx, queries, stepID, agentID, message)
	err = commands.FinishTx(tx, err)
	return
}

func markVerdict(ctx context.Context, repo commands.Repo, stepID string, agentID string, verdict string, message string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin poll", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	stored := sql.NullString{String: verdict, Valid: true}
	params := records.SetReviewerVerdictParams{
		Verdict: stored,
		ID:      agentID,
	}
	err = queries.SetReviewerVerdict(ctx, params)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	err = completedEvent(ctx, queries, stepID, agentID, message)
	err = commands.FinishTx(tx, err)
	return
}

func markFixed(ctx context.Context, repo commands.Repo, stepID string, agentID string, workID string, sha string, message string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin poll", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	head := sql.NullString{String: sha, Valid: true}
	params := records.MarkWorkFixedParams{
		FixerHeadSha: head,
		ID:           workID,
	}
	err = queries.MarkWorkFixed(ctx, params)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	err = completedEvent(ctx, queries, stepID, agentID, message)
	err = commands.FinishTx(tx, err)
	return
}

func failAgent(ctx context.Context, repo commands.Repo, stepID string, agentID string, reason string) (line string, err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin poll", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	eventID, err := newEventID()
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	params := records.CreateEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    eventFailed,
		Message: reason,
	}
	err = queries.CreateEvent(ctx, params)
	err = commands.FinishTx(tx, err)
	if err != nil {
		return
	}
	line = reason
	return
}

func completedEvent(ctx context.Context, queries *records.Queries, stepID string, agentID string, message string) (err error) {
	eventID, err := newEventID()
	if err != nil {
		return
	}
	params := records.CreateEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    eventCompleted,
		Message: message,
	}
	err = queries.CreateEvent(ctx, params)
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
