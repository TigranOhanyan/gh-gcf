package executor

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"charm.land/log/v2"
	"github.com/oklog/ulid/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	stateentities "github.com/TigranOhanyan/gh-gcf/statedb/entities"
)

// Action is the work one command does for a single app repo.
// Name is the action name in the state commit subject.
// Apply returns the message for that subject, formed from what the action did.
// When Apply returns an error, the subject uses that error text instead.
type Action interface {
	Name() (name string)
	Apply(ctx context.Context, repo commands.Repo, tick stateentities.Tick) (message string, err error)
}

// StepExecutor carries one action across every app repo in FactoryDir.
// ConfigBranch and StateBranch are the branches it opens.
// TickID is the tick this run records. It is required.
type StepExecutor struct {
	FactoryDir   string
	ConfigBranch string
	StateBranch  string
	TickID       string
}

// Execute opens every app repo, records one step, applies action, and commits state.db.
// Config is opened from ConfigBranch and is not committed. State is opened from StateBranch.
// TickID is the tick already stored for this run. Execute does not insert a tick.
// The step id is new for this call and the same in every repo. Its name is the action name.
// The commit subject is [tick id] [action name] [owner/name] and the message Apply returned.
// When Apply returns an error, the message is that error text and the state commit still happens.
// When the step insert fails, the subject is empty and that repo is not committed.
// The first action error stops the walk. Repos already committed stay committed.
func (step StepExecutor) Execute(ctx context.Context, action Action) (err error) {
	missingTick := step.TickID == ""
	if missingTick {
		err = fmt.Errorf("tick id is required")
		log.Error("tick id", "err", err)
		return
	}

	name := action.Name()
	log.Info("step", "name", name)

	stepID, err := newStepID()
	if err != nil {
		return
	}

	visit := func(ctx context.Context, repo commands.Repo) (subject string, err error) {
		err = insertStep(ctx, repo, stepID, step.TickID, name)
		if err != nil {
			return
		}

		tick := stateentities.Tick{ID: step.TickID}
		message, err := action.Apply(ctx, repo, tick)
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
		subject = commands.CommitSubject(step.TickID, name, repoName, message)
		return
	}
	err = commands.VisitRepos(ctx, step.FactoryDir, step.ConfigBranch, step.StateBranch, visit)
	return
}

func insertStep(ctx context.Context, repo commands.Repo, id string, tickID string, name string) (err error) {
	params := stateentities.CreateStepParams{
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
