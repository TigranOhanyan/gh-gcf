package statedb

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const stepID = "01ARZ3NDEKTSV4RRFFQ69G5FB0"
const eventID = "01ARZ3NDEKTSV4RRFFQ69G5FB1"
const agentID = "bc-018f6b2c-7c3a-7b2e-8a1d-5c3e9f0a1b2d"
const workID = "01ARZ3NDEKTSV4RRFFQ69G5FB2"

func Test_CreateEvent_should_store_the_event(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(JunkDir, "Test_CreateEvent_should_store_the_event.db")
	removeDatabase(t, path)

	db, err := Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	err = db.Queries.CreateTick(ctx, tickID)
	require.NoError(t, err)
	step := records.CreateStepParams{
		ID:     stepID,
		TickID: tickID,
		Name:   "run-cycles",
	}
	err = db.Queries.CreateStep(ctx, step)
	require.NoError(t, err)
	seedAgent(t, db)

	startOfTest := time.Now().Add(-time.Second)
	event := records.CreateEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    "starting",
		Message: "claimed",
	}
	err = db.Queries.CreateEvent(ctx, event)
	require.NoError(t, err)
	startOfCheck := time.Now().Add(time.Second)

	rows, err := db.Queries.GetEvents(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	row := rows[0]
	assert.Equal(t, eventID, row.ID)
	assert.Equal(t, stepID, row.StepID)
	assert.Equal(t, "starting", row.Kind)
	assert.Equal(t, "claimed", row.Message)
	createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
	require.NoError(t, err)
	assert.WithinRange(t, createdAt, startOfTest, startOfCheck)
}

func Test_CreateEvent_should_store_an_empty_message(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(JunkDir, "Test_CreateEvent_should_store_an_empty_message.db")
	removeDatabase(t, path)

	db, err := Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	err = db.Queries.CreateTick(ctx, tickID)
	require.NoError(t, err)
	step := records.CreateStepParams{
		ID:     stepID,
		TickID: tickID,
		Name:   "run-cycles",
	}
	err = db.Queries.CreateStep(ctx, step)
	require.NoError(t, err)
	seedAgent(t, db)

	event := records.CreateEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    "ack",
		Message: "",
	}
	err = db.Queries.CreateEvent(ctx, event)
	require.NoError(t, err)

	rows, err := db.Queries.GetEvents(ctx)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Empty(t, rows[0].Message)
}

func Test_CreateEvent_should_refuse_an_id_that_is_not_26_characters(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(JunkDir, "Test_CreateEvent_should_refuse_an_id_that_is_not_26_characters.db")
	removeDatabase(t, path)

	db, err := Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	err = db.Queries.CreateTick(ctx, tickID)
	require.NoError(t, err)
	step := records.CreateStepParams{
		ID:     stepID,
		TickID: tickID,
		Name:   "run-cycles",
	}
	err = db.Queries.CreateStep(ctx, step)
	require.NoError(t, err)
	seedAgent(t, db)

	event := records.CreateEventParams{
		ID:      "short",
		StepID:  stepID,
		AgentID: agentID,
		Kind:    "starting",
		Message: "claimed",
	}
	err = db.Queries.CreateEvent(ctx, event)
	require.Error(t, err)

	rows, err := db.Queries.GetEvents(ctx)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func Test_CreateEvent_should_refuse_an_unknown_kind(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(JunkDir, "Test_CreateEvent_should_refuse_an_unknown_kind.db")
	removeDatabase(t, path)

	db, err := Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	err = db.Queries.CreateTick(ctx, tickID)
	require.NoError(t, err)
	step := records.CreateStepParams{
		ID:     stepID,
		TickID: tickID,
		Name:   "run-cycles",
	}
	err = db.Queries.CreateStep(ctx, step)
	require.NoError(t, err)
	seedAgent(t, db)

	event := records.CreateEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    "running",
		Message: "still going",
	}
	err = db.Queries.CreateEvent(ctx, event)
	require.Error(t, err)

	rows, err := db.Queries.GetEvents(ctx)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func Test_CreateEvent_should_refuse_a_missing_step(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(JunkDir, "Test_CreateEvent_should_refuse_a_missing_step.db")
	removeDatabase(t, path)

	db, err := Open(ctx, path)
	require.NoError(t, err)
	defer db.Close()

	event := records.CreateEventParams{
		ID:      eventID,
		StepID:  stepID,
		AgentID: agentID,
		Kind:    "starting",
		Message: "claimed",
	}
	err = db.Queries.CreateEvent(ctx, event)
	require.Error(t, err)

	rows, err := db.Queries.GetEvents(ctx)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func seedAgent(t *testing.T, db *DB) {
	t.Helper()
	ctx := t.Context()
	member := records.CreateMemberParams{
		Name:              "features",
		Role:              "builder",
		AttentionLabel:    "attention",
		InstructionSha256: "abc",
	}
	err := db.Queries.CreateMember(ctx, member)
	require.NoError(t, err)
	builder := records.CreateBuilderMemberParams{
		Name:          "features",
		StartingLabel: "todo",
	}
	err = db.Queries.CreateBuilderMember(ctx, builder)
	require.NoError(t, err)
	issue := records.CreateIssueParams{
		Number: 7,
		Url:    "https://github.com/octo/app/issues/7",
	}
	err = db.Queries.CreateIssue(ctx, issue)
	require.NoError(t, err)
	work := records.CreateCodingWorkParams{
		ID:          workID,
		IssueNumber: 7,
		BuilderName: "features",
	}
	err = db.Queries.CreateCodingWork(ctx, work)
	require.NoError(t, err)
	agent := records.CreateAgentParams{
		ID:         agentID,
		Role:       "builder",
		WorkID:     workID,
		MemberName: "features",
	}
	err = db.Queries.CreateAgent(ctx, agent)
	require.NoError(t, err)
	err = db.Queries.CreateBuilderAgent(ctx, agentID)
	require.NoError(t, err)
}
