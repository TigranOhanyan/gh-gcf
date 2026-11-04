package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"charm.land/log/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const stepName = "app"

type sight int

const (
	sightMissing sight = iota
	sightSuspended
	sightInstalled
)

type appStep struct{}

func (appStep) Name() (name string) {
	name = stepName
	return
}

func (step appStep) Apply(ctx context.Context, repo commands.Repo, _ records.Tick, _ records.Step) (message string, err error) {
	ids, err := repo.Config.Queries.ListGithubApps(ctx)
	if err != nil {
		log.Error("list apps", "owner", repo.Owner, "name", repo.Name, "err", err)
		return
	}

	var lines []string
	for _, id := range ids {
		line, callErr := step.ensure(ctx, repo, id)
		hasLine := line != ""
		if hasLine {
			lines = append(lines, line)
		}
		failed := callErr != nil
		if failed {
			message = strings.Join(lines, "\n")
			text := callErr.Error()
			err = fmt.Errorf("[%d] %s", id, text)
			return
		}
	}
	message = strings.Join(lines, "\n")
	return
}

func (step appStep) ensure(ctx context.Context, repo commands.Repo, id int64) (line string, err error) {
	stored, err := repo.State.Queries.GetGithubApp(ctx, id)
	absent := errors.Is(err, sql.ErrNoRows)
	lookupFailed := err != nil && !absent
	if lookupFailed {
		log.Error("get app", "owner", repo.Owner, "name", repo.Name, "app", id, "err", err)
		return
	}
	err = nil

	seen, clientID, installationID, err := step.lookUp(ctx, repo.Owner, repo.Name, id)
	if err != nil {
		return
	}

	usable := seen == sightInstalled
	if !usable {
		err = absentInstallation(seen)
		return
	}

	if absent {
		err = insertApp(ctx, repo, id, clientID, installationID)
		if err != nil {
			return
		}
		line = fmt.Sprintf("[%d] recorded", id)
		return
	}

	same := stored.InstallationID == installationID
	if same {
		return
	}

	err = updateApp(ctx, repo, id, clientID, installationID)
	if err != nil {
		return
	}
	line = fmt.Sprintf("[%d] installation updated", id)
	return
}

func absentInstallation(seen sight) (err error) {
	suspended := seen == sightSuspended
	if suspended {
		err = fmt.Errorf("installation is suspended")
		return
	}
	err = fmt.Errorf("installation is missing")
	return
}

func insertApp(ctx context.Context, repo commands.Repo, id int64, clientID string, installationID int64) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin app", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	params := records.CreateGithubAppParams{
		AppID:          id,
		ClientID:       clientID,
		InstallationID: installationID,
	}
	err = queries.CreateGithubApp(ctx, params)
	err = finishTx(tx, err)
	return
}

func updateApp(ctx context.Context, repo commands.Repo, id int64, clientID string, installationID int64) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin app", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	params := records.UpdateGithubAppParams{
		ClientID:       clientID,
		InstallationID: installationID,
		AppID:          id,
	}
	err = queries.UpdateGithubApp(ctx, params)
	err = finishTx(tx, err)
	return
}
