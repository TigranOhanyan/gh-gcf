package executor

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/oklog/ulid/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

// Step is the work one command does for a single app repo.
// Name is the step name in the state commit subject.
// Apply returns the lines the commit body records, and an error when the step stopped.
// Both can be set. The executor keeps each of them.
type Step interface {
	Name() (name string)
	Apply(ctx context.Context, repo commands.Repo, tick records.Tick, step records.Step) (message string, err error)
}

// StepExecutor carries one step across every app repo in FactoryDir.
// ConfigBranch and StateBranch are the branches it opens.
// TickID is the tick this run records. It is required.
type StepExecutor struct {
	FactoryDir   string
	ConfigBranch string
	StateBranch  string
	TickID       string
}

// Execute opens every app repo, records one step, applies the step, and commits state.db.
// Config is opened from ConfigBranch and is not committed. State is opened from StateBranch.
// TickID is the tick already stored for this run. Execute does not insert a tick.
// The step id is new for this call and the same in every repo. Its name is the step name.
// The commit subject is ✅ or ❌, then [tick id] [step name] [owner/name].
// ✅ means Apply returned no error. ❌ means it did.
// The body is Apply's message, with ✅ at the start of each line.
// When Apply also returns an error, that text follows, with ❌ at the start of each line.
// The state commit still happens. When the step insert fails, the subject is empty and that repo is not committed.
// The first step error stops the walk. Repos already committed stay committed.
func (exec StepExecutor) Execute(ctx context.Context, step Step) (err error) {
	missingTick := exec.TickID == ""
	if missingTick {
		err = fmt.Errorf("tick id is required")
		log.Error("tick id", "err", err)
		return
	}

	name := step.Name()
	log.Info("step", "name", name)

	stepID, err := newStepID()
	if err != nil {
		return
	}

	visit := func(ctx context.Context, repo commands.Repo) (commit commands.StateCommit, err error) {
		err = insertStep(ctx, repo, stepID, exec.TickID, name)
		if err != nil {
			return
		}

		tick := records.Tick{ID: exec.TickID}
		stepRow := records.Step{
			ID:     stepID,
			TickID: exec.TickID,
			Name:   name,
		}
		message, err := step.Apply(ctx, repo, tick, stepRow)
		repoName := repo.Owner + "/" + repo.Name
		failed := err != nil
		if failed {
			log.Error("apply", "name", name, "repo", repoName, "err", err)
		}
		succeeded := !failed
		if succeeded {
			log.Info("apply", "name", name, "repo", repoName, "message", message)
		}
		commit.Subject = stepSubject(failed, exec.TickID, name, repoName)
		commit.Body = markedBody(message, err)
		return
	}
	err = commands.VisitRepos(ctx, exec.FactoryDir, exec.ConfigBranch, exec.StateBranch, visit)
	return
}

func insertStep(ctx context.Context, repo commands.Repo, id string, tickID string, name string) (err error) {
	params := records.CreateStepParams{
		ID:     id,
		TickID: tickID,
		Name:   name,
	}
	err = repo.State.Queries.CreateStep(ctx, params)
	if err != nil {
		log.Error("insert step", "owner", repo.Owner, "name", repo.Name, "err", err)
		return
	}
	log.Info("insert step", "owner", repo.Owner, "name", repo.Name, "id", id)
	return
}

const passedMark = "✅"
const failedMark = "❌"

func stepSubject(failed bool, id string, name string, repoName string) (subject string) {
	mark := passedMark
	if failed {
		mark = failedMark
	}
	subject = fmt.Sprintf("%s [%s] [%s] [%s]", mark, id, name, repoName)
	return
}

func markedBody(message string, err error) (body string) {
	success := markLines(message, passedMark)
	var failure string
	if err != nil {
		failure = markLines(err.Error(), failedMark)
	}
	body = joinLines(success, failure)
	return
}

func markLines(text string, mark string) (marked string) {
	if text == "" {
		return
	}
	lines := strings.Split(text, "\n")
	markedLines := make([]string, 0, len(lines))
	for _, line := range lines {
		markedLine := mark + " " + line
		markedLines = append(markedLines, markedLine)
	}
	marked = strings.Join(markedLines, "\n")
	return
}

func joinLines(left string, right string) (joined string) {
	if left == "" {
		joined = right
		return
	}
	if right == "" {
		joined = left
		return
	}
	joined = left + "\n" + right
	return
}

func newStepID() (id string, err error) {
	now := time.Now()
	timestamp := ulid.Timestamp(now)
	value, err := ulid.New(timestamp, rand.Reader)
	if err != nil {
		log.Error("new step id", "err", err)
		return
	}
	id = value.String()
	log.Info("step", "id", id)
	return
}
