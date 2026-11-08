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
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/run"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const stepName = "run_fixer"
const statusCompleted = "completed"
const roleFixer = "fixer"
const eventStarting = "starting"
const eventAck = "ack"
const eventRejected = "rejected"
const conclusionFailure = "failure"
const conclusionCancelled = "cancelled"
const conclusionTimedOut = "timed_out"

type checkStep struct {
	GitHub *github.Client
	app    commands.FactoryApp
	cursor run.CursorClient
}

func (checkStep) Name() (name string) {
	name = stepName
	return
}

func (step checkStep) Apply(ctx context.Context, repo commands.Repo, _ records.Tick, stepRow records.Step) (message string, err error) {
	works, err := repo.State.Queries.ListWorksReadyForChecks(ctx)
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

func (step checkStep) consider(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, work records.ListWorksReadyForChecksRow, noted map[string]struct{}) (line string, err error) {
	names, err := repo.State.Queries.ListDevelopmentCheckNames(ctx, work.DevelopmentName)
	if err != nil {
		log.Error("list checks", "development", work.DevelopmentName, "err", err)
		return
	}
	red := false
	listed := len(names) > 0
	if listed {
		sha := work.BuilderHeadSha.String
		invalid := !work.BuilderHeadSha.Valid
		empty := sha == ""
		missingHead := invalid || empty
		if missingHead {
			err = fmt.Errorf("builder head is missing")
			log.Error("builder head", "issue", work.IssueNumber, "err", err)
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
		red = listedChecksRed(names, runs)
	}
	line, err = step.readVerdicts(ctx, repo, stepID, client, work, red, noted)
	return
}

func (step checkStep) readVerdicts(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, work records.ListWorksReadyForChecksRow, red bool, noted map[string]struct{}) (line string, err error) {
	development, err := repo.State.Queries.GetDevelopment(ctx, work.DevelopmentName)
	if err != nil {
		log.Error("development", "development", work.DevelopmentName, "err", err)
		return
	}
	missingPull := !work.PullRequestNumber.Valid
	if missingPull {
		err = fmt.Errorf("pull request is missing")
		log.Error("pull request", "issue", work.IssueNumber, "err", err)
		return
	}
	number := int(work.PullRequestNumber.Int64)
	standardsVerdict, standardsFound, err := step.appVerdict(ctx, repo, number, development.StandardsReviewerApp)
	if err != nil {
		return
	}
	specificationVerdict, specificationFound, err := step.appVerdict(ctx, repo, number, development.SpecificationReviewerApp)
	if err != nil {
		return
	}
	missing := !standardsFound || !specificationFound
	if missing {
		reason := fmt.Sprintf("[%d] verdict is missing", work.IssueNumber)
		line, err = step.missingVerdict(ctx, repo, client, development, work, reason)
		return
	}
	log.Info("verdicts", "issue", work.IssueNumber, "standards", standardsVerdict, "specification", specificationVerdict)
	standardsChanges := standardsVerdict == verdictChanges
	specificationChanges := specificationVerdict == verdictChanges
	changes := standardsChanges || specificationChanges
	needsFix := changes || red
	if !needsFix {
		// Issue 32 sets the work to finished when both verdicts approve and every listed check passed.
		return
	}
	line, err = step.startFixer(ctx, repo, stepID, client, development, work, noted)
	return
}

func (step checkStep) startFixer(ctx context.Context, repo commands.Repo, stepID string, client *github.Client, development records.Development, work records.ListWorksReadyForChecksRow, noted map[string]struct{}) (line string, err error) {
	sum, err := commands.DevelopmentInstructionFingerprint(repo.Dir, development.Name, roleFixer)
	if err != nil {
		return
	}
	changed := sum != development.FixerInstructionSha256
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

	text, err := commands.ReadDevelopmentInstruction(repo.Dir, development.Name, roleFixer)
	if err != nil {
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
	promptBody := promptText(text, work.IssueNumber, work.IssueUrl, pullNumber)
	prompt := run.CursorPrompt{Text: promptBody}
	repoURL := fmt.Sprintf("https://github.com/%s/%s", repo.Owner, repo.Name)
	oneRepo := run.CursorRepo{URL: repoURL}
	repos := []run.CursorRepo{oneRepo}
	body := run.CursorCreate{
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

func claim(ctx context.Context, repo commands.Repo, stepID string, developmentName string, work records.ListWorksReadyForChecksRow, agentID string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin run", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	agentParams := records.CreateDevelopmentAgentParams{
		ID:   agentID,
		Role: roleFixer,
	}
	err = queries.CreateDevelopmentAgent(ctx, agentParams)
	if err != nil {
		err = commands.FinishTx(tx, err)
		return
	}
	link := sql.NullString{String: agentID, Valid: true}
	linkParams := records.SetFixerAgentParams{
		FixerAgent: link,
		ID:         work.WorkID,
	}
	err = queries.SetFixerAgent(ctx, linkParams)
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

func (step checkStep) appVerdict(ctx context.Context, repo commands.Repo, number int, appID int64) (verdict string, found bool, err error) {
	app, err := repo.State.Queries.GetGithubApp(ctx, appID)
	if err != nil {
		log.Error("github app", "app", appID, "err", err)
		return
	}
	verdict, found, err = reviewerVerdict(ctx, repo.Owner, repo.Name, number, app.AppID, app.InstallationID)
	return
}

func (step checkStep) missingVerdict(ctx context.Context, repo commands.Repo, client *github.Client, development records.Development, work records.ListWorksReadyForChecksRow, reason string) (line string, err error) {
	err = markAttention(ctx, repo, work.WorkID)
	if err != nil {
		return
	}
	number := int(work.IssueNumber)
	err = commands.AddAttention(ctx, client, repo.Owner, repo.Name, number, development.AttentionLabel)
	if err != nil {
		return
	}
	line = reason
	return
}

func markAttention(ctx context.Context, repo commands.Repo, workID string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin run", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	err = queries.SetDevelopmentWorkAttention(ctx, workID)
	err = commands.FinishTx(tx, err)
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
