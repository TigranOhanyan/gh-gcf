package tick

import (
	"context"
	"crypto/rand"
	"time"

	"go.uber.org/zap"

	"github.com/TigranOhanyan/gh-gcf/logging"
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
	id, err = newTickID(ctx)
	if err != nil {
		return
	}
	ctx = logging.With(ctx, zap.String("tick_id", id), zap.String("step", tickName))
	logging.From(ctx).Info("tick")

	visit := func(ctx context.Context, repo commands.Repo) (commit commands.StateCommit, err error) {
		ctx = logging.With(ctx, zap.String("owner", repo.Owner), zap.String("repo", repo.Name))
		err = repo.State.Queries.CreateTick(ctx, id)
		if err != nil {
			logging.From(ctx).Error("insert tick", zap.Any("owner", repo.Owner), zap.Any("name", repo.Name), zap.Error(err))
			return
		}
		logging.From(ctx).Info("insert tick", zap.Any("owner", repo.Owner), zap.Any("name", repo.Name))
		repoName := repo.Owner + "/" + repo.Name
		commit.Subject = commands.CommitSubject(id, tickName, repoName, tickCreated)
		return
	}
	err = commands.VisitRepos(ctx, tick.FactoryDir, tick.ConfigBranch, tick.StateBranch, visit)
	if err != nil {
		id = ""
		return
	}
	return
}

func newTickID(ctx context.Context) (id string, err error) {
	now := time.Now()
	timestamp := ulid.Timestamp(now)
	value, err := ulid.New(timestamp, rand.Reader)
	if err != nil {
		logging.From(ctx).Error("new tick id", zap.Error(err))
		return
	}
	id = value.String()
	return
}
