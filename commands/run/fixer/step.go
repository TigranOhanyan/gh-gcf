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
	none := len(names) == 0
	if none {
		return
	}
	sha := work.BuilderHeadSha.String
	invalid := !work.BuilderHeadSha.Valid
	empty := sha == ""
	missing := invalid || empty
	if missing {
		err = fmt.Errorf("builder head is missing")
		log.Error("builder head", "issue", work.IssueNumber, "err", err)
		return
	}
	runs, err := listCheckRuns(ctx, client, repo.Owner, repo.Name, sha)
	if err != nil {
		return
	}
	waiting := listedChecksWaiting(names, runs)
	if waiting {
		log.Info("checks waiting", "issue", work.IssueNumber)
	}
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
