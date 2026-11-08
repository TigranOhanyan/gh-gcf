package fixer

import (
	"context"
	"fmt"
	"strings"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const stepName = "run_fixer"
const statusCompleted = "completed"

type checkStep struct {
	GitHub *github.Client
	app    commands.FactoryApp
}

func (checkStep) Name() (name string) {
	name = stepName
	return
}

func (step checkStep) Apply(ctx context.Context, repo commands.Repo, _ records.Tick, _ records.Step) (message string, err error) {
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

	var lines []string
	for _, work := range works {
		var line string
		line, err = step.consider(ctx, repo, client, work)
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

func (step checkStep) consider(ctx context.Context, repo commands.Repo, client *github.Client, work records.ListWorksReadyForChecksRow) (line string, err error) {
	names, err := repo.State.Queries.ListDevelopmentCheckNames(ctx, work.DevelopmentName)
	if err != nil {
		log.Error("list checks", "development", work.DevelopmentName, "err", err)
		return
	}
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
	}
	line, err = step.readVerdicts(ctx, repo, client, work)
	return
}

func (step checkStep) readVerdicts(ctx context.Context, repo commands.Repo, client *github.Client, work records.ListWorksReadyForChecksRow) (line string, err error) {
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
	// Issue 31 starts the fixer when one verdict requests changes or a listed check is red.
	// Issue 32 sets the work to finished when both verdicts approve and every listed check passed.
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
	listed := map[string]struct{}{}
	for _, name := range names {
		listed[name] = struct{}{}
	}
	newest := map[string]*github.CheckRun{}
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
