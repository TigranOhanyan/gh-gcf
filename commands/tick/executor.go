package tick

import (
	"context"
	"crypto/rand"
	"time"

	"charm.land/log/v2"
	"github.com/oklog/ulid/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

const tickName = "tick"
const tickCreated = "created"

// tickExecutor stores one tick across every app repo in FactoryDir.
// ConfigBranch and StateBranch are the branches it opens.
type tickExecutor struct {
	FactoryDir   string
	ConfigBranch string
	StateBranch  string
}

// CreateTick stores one new tick id in every app repo and commits each state.db.
// created_at is set by the database to the current UTC time.
// Config is opened from ConfigBranch and is not committed. State is opened from StateBranch.
// The commit subject is [tick id] [tick] [owner/name] created.
// The same id is stored in every repo. An empty FactoryDir uses the current directory.
// The first error stops the walk. Repos already committed stay committed.
func (tick tickExecutor) CreateTick(ctx context.Context) (id string, err error) {
	id, err = newTickID()
	if err != nil {
		return
	}

	visit := func(ctx context.Context, repo commands.Repo) (subject string, err error) {
		err = repo.State.Queries.CreateTick(ctx, id)
		if err != nil {
			log.Error("insert tick", "owner", repo.Owner, "name", repo.Name, "err", err)
			return
		}
		log.Info("insert tick", "owner", repo.Owner, "name", repo.Name)
		repoName := repo.Owner + "/" + repo.Name
		subject = commands.CommitSubject(id, tickName, repoName, tickCreated)
		return
	}
	err = commands.VisitRepos(ctx, tick.FactoryDir, tick.ConfigBranch, tick.StateBranch, visit)
	if err != nil {
		id = ""
		return
	}
	return
}

func newTickID() (id string, err error) {
	now := time.Now()
	timestamp := ulid.Timestamp(now)
	value, err := ulid.New(timestamp, rand.Reader)
	if err != nil {
		log.Error("new tick id", "err", err)
		return
	}
	id = value.String()
	log.Info("tick", "id", id)
	return
}
