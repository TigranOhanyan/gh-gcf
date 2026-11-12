package label

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/TigranOhanyan/gh-gcf/logging"
	"github.com/google/go-github/v93/github"

	"github.com/TigranOhanyan/gh-gcf/commands"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const stepName = "label"

type labelStep struct {
	GitHub *github.Client
	app    commands.FactoryApp
}

func (labelStep) Name() (name string) {
	name = stepName
	return
}

func (step labelStep) Apply(ctx context.Context, repo commands.Repo, _ records.Tick, _ records.Step) (message string, err error) {
	labels, err := repo.Config.Queries.ListLabels(ctx)
	if err != nil {
		logging.From(ctx).Error("list labels", zap.Any("owner", repo.Owner), zap.Any("name", repo.Name), zap.Error(err))
		return
	}

	needsClient := len(labels) > 0
	if needsClient {
		client, clientErr := commands.GitHubClient(ctx, step.app, repo.Owner, repo.Name, step.GitHub)
		if clientErr != nil {
			err = clientErr
			return
		}
		step.GitHub = client
	}

	var lines []string
	for _, label := range labels {
		line, callErr := step.ensure(ctx, repo, label)
		hasLine := line != ""
		if hasLine {
			lines = append(lines, line)
		}
		failed := callErr != nil
		if failed {
			message = strings.Join(lines, "\n")
			err = fmt.Errorf("[%s] %s", label.Name, callErr.Error())
			return
		}
	}
	message = strings.Join(lines, "\n")
	return
}

func (step labelStep) ensure(ctx context.Context, repo commands.Repo, label configrecords.Label) (line string, err error) {
	_, err = repo.State.Queries.GetLabel(ctx, label.Name)
	absent := errors.Is(err, sql.ErrNoRows)
	lookupFailed := err != nil && !absent
	if lookupFailed {
		logging.From(ctx).Error("get label", zap.Any("owner", repo.Owner), zap.Any("name", repo.Name), zap.Any("label", label.Name), zap.Error(err))
		return
	}
	err = nil

	onGitHub, err := step.labelOnGitHub(ctx, repo.Owner, repo.Name, label.Name)
	if err != nil {
		return
	}

	alreadyBoth := onGitHub && !absent
	if alreadyBoth {
		return
	}
	needsRecord := onGitHub && absent
	if needsRecord {
		err = step.insertLabel(ctx, repo, label)
		return
	}

	err = step.beginCreate(ctx, repo, label, absent)
	if err != nil {
		return
	}
	err = step.createOnGitHub(ctx, repo.Owner, repo.Name, label)
	if err != nil {
		return
	}
	err = step.finishCreate(ctx, repo, label.Name)
	if err != nil {
		return
	}
	line = fmt.Sprintf("[%s] created", label.Name)
	return
}

func (step labelStep) labelOnGitHub(ctx context.Context, owner string, repoName string, name string) (exists bool, err error) {
	_, resp, err := step.GitHub.Issues.GetLabel(ctx, owner, repoName, name)
	commands.CloseGitHub(resp)
	if err == nil {
		exists = true
		return
	}
	missing := commands.GitHubNotFound(err)
	if missing {
		err = nil
		return
	}
	logging.From(ctx).Error("get github label", zap.Any("owner", owner), zap.Any("name", repoName), zap.Any("label", name), zap.Error(err))
	return
}

func (step labelStep) createOnGitHub(ctx context.Context, owner string, repoName string, label configrecords.Label) (err error) {
	color := label.Color
	request := github.CreateIssueLabelRequest{
		Name:  label.Name,
		Color: &color,
	}
	_, resp, err := step.GitHub.Issues.CreateLabel(ctx, owner, repoName, request)
	commands.CloseGitHub(resp)
	if err != nil {
		logging.From(ctx).Error("create github label", zap.Any("owner", owner), zap.Any("name", repoName), zap.Any("label", label.Name), zap.Error(err))
	}
	return
}

func (step labelStep) insertLabel(ctx context.Context, repo commands.Repo, label configrecords.Label) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		logging.From(ctx).Error("begin label", zap.Error(err))
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	params := records.CreateLabelParams{
		Name:  label.Name,
		Color: label.Color,
	}
	err = queries.CreateLabel(ctx, params)
	err = commands.FinishTx(tx, err)
	return
}

func (step labelStep) beginCreate(ctx context.Context, repo commands.Repo, label configrecords.Label, absent bool) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		logging.From(ctx).Error("begin label", zap.Error(err))
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	if absent {
		params := records.CreateLabelParams{
			Name:  label.Name,
			Color: label.Color,
		}
		err = queries.CreateLabel(ctx, params)
		if err != nil {
			err = commands.FinishTx(tx, err)
			return
		}
	}
	err = commands.FinishTx(tx, err)
	return
}

func (step labelStep) finishCreate(ctx context.Context, repo commands.Repo, name string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		logging.From(ctx).Error("begin label", zap.Error(err))
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	createdAt := sql.NullString{
		String: nowUTC(),
		Valid:  true,
	}
	params := records.SetLabelCreatedAtParams{
		CreatedAt: createdAt,
		Name:      name,
	}
	err = queries.SetLabelCreatedAt(ctx, params)
	err = commands.FinishTx(tx, err)
	return
}

func nowUTC() (text string) {
	text = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	return
}
