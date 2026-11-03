package label

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/google/go-github/v93/github"
	"github.com/oklog/ulid/v2"

	"github.com/TigranOhanyan/gh-gcf/commands"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const stepName = "label"
const eventStarting = "starting"
const eventCompleted = "completed"
const eventFailed = "failed"

type labelStep struct {
	GitHub *github.Client
}

func (labelStep) Name() (name string) {
	name = stepName
	return
}

func (step labelStep) Apply(ctx context.Context, repo commands.Repo, _ records.Tick, stepRow records.Step) (message string, err error) {
	labels, err := repo.Config.Queries.ListLabels(ctx)
	if err != nil {
		log.Error("list labels", "owner", repo.Owner, "name", repo.Name, "err", err)
		return
	}

	var lines []string
	for _, label := range labels {
		line, callErr := step.ensure(ctx, repo, stepRow, label)
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

func (step labelStep) ensure(ctx context.Context, repo commands.Repo, stepRow records.Step, label configrecords.Label) (line string, err error) {
	_, err = repo.State.Queries.GetLabel(ctx, label.Name)
	absent := errors.Is(err, sql.ErrNoRows)
	lookupFailed := err != nil && !absent
	if lookupFailed {
		log.Error("get label", "owner", repo.Owner, "name", repo.Name, "label", label.Name, "err", err)
		return
	}
	err = nil

	onGitHub, err := step.labelOnGitHub(ctx, repo.Owner, repo.Name, label.Name)
	if err != nil {
		recordErr := step.recordFailure(ctx, repo, stepRow.ID, err)
		if recordErr != nil {
			err = recordErr
			return
		}
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

	err = step.beginCreate(ctx, repo, stepRow.ID, label, absent)
	if err != nil {
		return
	}
	err = step.createOnGitHub(ctx, repo.Owner, repo.Name, label)
	if err != nil {
		recordErr := step.recordFailure(ctx, repo, stepRow.ID, err)
		if recordErr != nil {
			err = recordErr
			return
		}
		return
	}
	repoName := repo.Owner + "/" + repo.Name
	err = step.finishCreate(ctx, repo, stepRow.ID, repoName, label.Name)
	if err != nil {
		return
	}
	line = fmt.Sprintf("[%s] created", label.Name)
	return
}

func (step labelStep) labelOnGitHub(ctx context.Context, owner string, repoName string, name string) (exists bool, err error) {
	_, resp, err := step.GitHub.Issues.GetLabel(ctx, owner, repoName, name)
	closeGitHub(resp)
	if err == nil {
		exists = true
		return
	}
	missing := labelMissing(err)
	if missing {
		err = nil
		return
	}
	log.Error("get github label", "owner", owner, "name", repoName, "label", name, "err", err)
	return
}

func (step labelStep) createOnGitHub(ctx context.Context, owner string, repoName string, label configrecords.Label) (err error) {
	color := label.Color
	request := github.CreateIssueLabelRequest{
		Name:  label.Name,
		Color: &color,
	}
	_, resp, err := step.GitHub.Issues.CreateLabel(ctx, owner, repoName, request)
	closeGitHub(resp)
	if err != nil {
		log.Error("create github label", "owner", owner, "name", repoName, "label", label.Name, "err", err)
	}
	return
}

func (step labelStep) insertLabel(ctx context.Context, repo commands.Repo, label configrecords.Label) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin label", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	params := records.CreateLabelParams{
		Name:  label.Name,
		Color: label.Color,
	}
	err = queries.CreateLabel(ctx, params)
	err = finishTx(tx, err)
	return
}

func (step labelStep) beginCreate(ctx context.Context, repo commands.Repo, stepID string, label configrecords.Label, absent bool) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin label", "err", err)
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
			err = finishTx(tx, err)
			return
		}
	}
	repoName := repo.Owner + "/" + repo.Name
	message := creatingMessage(repoName, label.Name)
	err = insertEvent(ctx, queries, stepID, eventStarting, message)
	err = finishTx(tx, err)
	return
}

func (step labelStep) finishCreate(ctx context.Context, repo commands.Repo, stepID string, repoName string, name string) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin label", "err", err)
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
	if err != nil {
		err = finishTx(tx, err)
		return
	}
	message := createdMessage(repoName, name)
	err = insertEvent(ctx, queries, stepID, eventCompleted, message)
	err = finishTx(tx, err)
	return
}

func (step labelStep) recordFailure(ctx context.Context, repo commands.Repo, stepID string, callErr error) (err error) {
	tx, err := repo.State.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Error("begin label", "err", err)
		return
	}
	queries := repo.State.Queries.WithTx(tx)
	err = insertEvent(ctx, queries, stepID, eventFailed, callErr.Error())
	err = finishTx(tx, err)
	return
}

func insertEvent(ctx context.Context, queries *records.Queries, stepID string, kind string, message string) (err error) {
	id, err := newEventID()
	if err != nil {
		return
	}
	params := records.CreateEventParams{
		ID:      id,
		StepID:  stepID,
		Kind:    kind,
		Message: message,
	}
	err = queries.CreateEvent(ctx, params)
	if err != nil {
		log.Error("insert event", "kind", kind, "err", err)
	}
	return
}

func finishTx(tx *sql.Tx, workErr error) (err error) {
	failed := workErr != nil
	if failed {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Error("rollback", "err", rollbackErr)
		}
		err = workErr
		return
	}
	err = tx.Commit()
	if err != nil {
		log.Error("commit", "err", err)
	}
	return
}

func creatingMessage(repoName string, name string) (message string) {
	message = fmt.Sprintf("[%s] [%s] creating label", repoName, name)
	return
}

func createdMessage(repoName string, name string) (message string) {
	message = fmt.Sprintf("[%s] [%s] label is created", repoName, name)
	return
}

func nowUTC() (text string) {
	text = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
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
