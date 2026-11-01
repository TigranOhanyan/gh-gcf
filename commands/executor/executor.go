package executor

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"charm.land/log/v2"
	"github.com/oklog/ulid/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

// Step is the work one command does for a single app repo.
// Name is the step name in the state commit subject.
// Apply returns the message for that subject, formed from what the step did.
// When Apply returns an error, the subject uses that error text instead.
type Step interface {
	Name() (name string)
	Apply(ctx context.Context, repo commands.Repo, tick records.Tick) (message string, err error)
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
// The commit subject is [tick id] [step name] [owner/name] and the message Apply returned.
// When Apply returns an error, the message is that error text and the state commit still happens.
// When the step insert fails, the subject is empty and that repo is not committed.
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

	visit := func(ctx context.Context, repo commands.Repo) (subject string, err error) {
		err = insertStep(ctx, repo, stepID, exec.TickID, name)
		if err != nil {
			return
		}

		tick := records.Tick{ID: exec.TickID}
		message, err := step.Apply(ctx, repo, tick)
		repoName := repo.Owner + "/" + repo.Name
		failed := err != nil
		if failed {
			message = err.Error()
			log.Error("apply", "name", name, "repo", repoName, "err", err)
		}
		succeeded := !failed
		if succeeded {
			log.Info("apply", "name", name, "repo", repoName, "message", message)
		}
		subject = commands.CommitSubject(exec.TickID, name, repoName, message)
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
