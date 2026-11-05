package compile_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/commands/compile"
	"github.com/TigranOhanyan/gh-gcf/commands/tick"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const owner = "octo"
const repoName = "app"
const appID int64 = 123

func Test_Compile_should_record_a_builder_and_its_members(t *testing.T) {
	spec := builderSpec{
		name:      "features",
		starting:  "todo",
		attention: "attention",
		reviewer:  reviewerSpec{name: "standards", attention: "attention", app: appID},
		fixer:     fixerSpec{name: "fix", attention: "attention"},
		check:     "ci / unit",
		hasFixer:  true,
		hasReview: true,
	}
	repoDir := setupFactory(t, spec)
	id := runTick(t, repoDir)

	err := runCompile(t, repoDir, id)
	require.NoError(t, err)

	subject := mustGit(t, repoDir, "log", "-1", "--format=%s", "state")
	expectedSubject := fmt.Sprintf("✅ [%s] [compile] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log", "-1", "--format=%b", "state")
	expectedBody := strings.Join([]string{
		"✅ [features] recorded",
		"✅ [standards] recorded",
		"✅ [fix] recorded",
		"✅ [features] [ci / unit] recorded",
	}, "\n")
	assert.Equal(t, expectedBody, strings.TrimSpace(string(body)))

	db := openStateTip(t, repoDir)
	defer db.Close()
	builders := listBuilders(t, db)
	require.Len(t, builders, 1)
	assert.Equal(t, "features", builders[0].Name)
	assert.Equal(t, "builder", builders[0].Role)
	assert.Equal(t, "attention", builders[0].AttentionLabel)
	assert.Equal(t, "todo", builders[0].StartingLabel)

	reviewers := listReviewers(t, db)
	require.Len(t, reviewers, 1)
	assert.Equal(t, "standards", reviewers[0].Name)
	assert.Equal(t, "reviewer", reviewers[0].Role)
	assert.Equal(t, "attention", reviewers[0].AttentionLabel)
	assert.Equal(t, "features", reviewers[0].BuilderName)
	assert.Equal(t, appID, reviewers[0].GithubApp)

	fixers := listFixers(t, db)
	require.Len(t, fixers, 1)
	assert.Equal(t, "fix", fixers[0].Name)
	assert.Equal(t, "fixer", fixers[0].Role)
	assert.Equal(t, "attention", fixers[0].AttentionLabel)
	assert.Equal(t, "features", fixers[0].BuilderName)

	checks := listChecks(t, db)
	require.Len(t, checks, 1)
	assert.Equal(t, "features", checks[0].BuilderName)
	assert.Equal(t, "ci / unit", checks[0].Name)
}

func Test_Compile_should_leave_an_unchanged_builder(t *testing.T) {
	spec := builderSpec{
		name:      "features",
		starting:  "todo",
		attention: "attention",
	}
	repoDir := setupFactory(t, spec)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)

	err = runCompile(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log", "-1", "--format=%b", "state")
	assert.Empty(t, strings.TrimSpace(string(body)))
	db := openStateTip(t, repoDir)
	defer db.Close()
	builders := listBuilders(t, db)
	require.Len(t, builders, 1)
	assert.Equal(t, "features", builders[0].Name)
}

func Test_Compile_should_update_a_changed_attention_label(t *testing.T) {
	spec := builderSpec{
		name:      "features",
		starting:  "todo",
		attention: "attention",
		extra:     []string{"look"},
	}
	repoDir := setupFactory(t, spec)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	setAttention(t, repoDir, "features", "look")

	err = runCompile(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log", "-1", "--format=%b", "state")
	assert.Equal(t, "✅ [features] updated", strings.TrimSpace(string(body)))
	db := openStateTip(t, repoDir)
	defer db.Close()
	builders := listBuilders(t, db)
	require.Len(t, builders, 1)
	assert.Equal(t, "look", builders[0].AttentionLabel)
	assert.Equal(t, "todo", builders[0].StartingLabel)
}

func Test_Compile_should_keep_a_member_config_no_longer_has(t *testing.T) {
	spec := builderSpec{
		name:      "features",
		starting:  "todo",
		attention: "attention",
	}
	repoDir := setupFactory(t, spec)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	clearMembers(t, repoDir)

	err = runCompile(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log", "-1", "--format=%b", "state")
	assert.Empty(t, strings.TrimSpace(string(body)))
	db := openStateTip(t, repoDir)
	defer db.Close()
	builders := listBuilders(t, db)
	require.Len(t, builders, 1)
	assert.Equal(t, "features", builders[0].Name)
	assert.Equal(t, "todo", builders[0].StartingLabel)
	assert.Equal(t, "attention", builders[0].AttentionLabel)
}

func Test_Compile_should_require_a_tick_id(t *testing.T) {
	cmd := compile.NewCompile()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tick")
}

type reviewerSpec struct {
	name      string
	attention string
	app       int64
}

type fixerSpec struct {
	name      string
	attention string
}

type builderSpec struct {
	name      string
	starting  string
	attention string
	reviewer  reviewerSpec
	fixer     fixerSpec
	check     string
	hasReview bool
	hasFixer  bool
	extra     []string
}

func setupFactory(t *testing.T, spec builderSpec) (repoDir string) {
	t.Helper()
	ctx := t.Context()
	repoDir = filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)

	_ = mustGit(t, repoDir, "init", "-b", "main")
	_ = mustGit(t, repoDir, "config", "user.email", "test@example.com")
	_ = mustGit(t, repoDir, "config", "user.name", "test")

	configPath := filepath.Join(repoDir, "repos", owner, repoName, "config.db")
	err = os.MkdirAll(filepath.Dir(configPath), 0755)
	require.NoError(t, err)
	configDB, err := configdb.OpenAndMigrate(ctx, configPath)
	require.NoError(t, err)
	seedConfig(t, configDB, spec)
	err = configDB.Close()
	require.NoError(t, err)
	configRel := filepath.Join("repos", owner, repoName, "config.db")
	_ = mustGit(t, repoDir, "add", configRel)
	_ = mustGit(t, repoDir, "commit", "-m", "config")

	_ = mustGit(t, repoDir, "checkout", "-b", "state")
	statePath := filepath.Join(repoDir, "repos", owner, repoName, "state.db")
	stateDB, err := statedb.Open(ctx, statePath)
	require.NoError(t, err)
	err = stateDB.Close()
	require.NoError(t, err)
	stateRel := filepath.Join("repos", owner, repoName, "state.db")
	_ = mustGit(t, repoDir, "add", stateRel)
	_ = mustGit(t, repoDir, "commit", "-m", "state")
	_ = mustGit(t, repoDir, "checkout", "main")
	return
}

func seedConfig(t *testing.T, db *configdb.DB, spec builderSpec) {
	t.Helper()
	ctx := t.Context()
	labels := []string{spec.starting, spec.attention}
	labels = append(labels, spec.extra...)
	if spec.hasReview {
		labels = append(labels, spec.reviewer.attention)
	}
	if spec.hasFixer {
		labels = append(labels, spec.fixer.attention)
	}
	seen := map[string]bool{}
	for _, name := range labels {
		if seen[name] {
			continue
		}
		seen[name] = true
		params := configrecords.CreateLabelParams{
			Name:  name,
			Color: "ededed",
		}
		err := db.Queries.CreateLabel(ctx, params)
		require.NoError(t, err)
	}
	if spec.hasReview {
		err := db.Queries.CreateGithubApp(ctx, spec.reviewer.app)
		require.NoError(t, err)
	}

	member := configrecords.CreateMemberParams{
		Name:           spec.name,
		Role:           "builder",
		AttentionLabel: spec.attention,
	}
	err := db.Queries.CreateMember(ctx, member)
	require.NoError(t, err)
	builder := configrecords.CreateBuilderMemberParams{
		Name:          spec.name,
		StartingLabel: spec.starting,
	}
	err = db.Queries.CreateBuilderMember(ctx, builder)
	require.NoError(t, err)

	if spec.hasReview {
		reviewMember := configrecords.CreateMemberParams{
			Name:           spec.reviewer.name,
			Role:           "reviewer",
			AttentionLabel: spec.reviewer.attention,
		}
		err = db.Queries.CreateMember(ctx, reviewMember)
		require.NoError(t, err)
		reviewer := configrecords.CreateReviewerMemberParams{
			Name:        spec.reviewer.name,
			BuilderName: spec.name,
			GithubApp:   spec.reviewer.app,
		}
		err = db.Queries.CreateReviewerMember(ctx, reviewer)
		require.NoError(t, err)
	}
	if spec.hasFixer {
		fixMember := configrecords.CreateMemberParams{
			Name:           spec.fixer.name,
			Role:           "fixer",
			AttentionLabel: spec.fixer.attention,
		}
		err = db.Queries.CreateMember(ctx, fixMember)
		require.NoError(t, err)
		fixer := configrecords.CreateFixerMemberParams{
			Name:        spec.fixer.name,
			BuilderName: spec.name,
		}
		err = db.Queries.CreateFixerMember(ctx, fixer)
		require.NoError(t, err)
	}
	if spec.check != "" {
		check := configrecords.CreateGithubCheckParams{
			BuilderName: spec.name,
			Name:        spec.check,
		}
		err = db.Queries.CreateGithubCheck(ctx, check)
		require.NoError(t, err)
	}
}

func setAttention(t *testing.T, repoDir string, name string, attention string) {
	t.Helper()
	configPath := filepath.Join(repoDir, "repos", owner, repoName, "config.db")
	db, err := configdb.OpenAndMigrate(t.Context(), configPath)
	require.NoError(t, err)
	_, err = db.DB.ExecContext(t.Context(), `UPDATE member SET attention_label = ? WHERE name = ?`, attention, name)
	require.NoError(t, err)
	err = db.Close()
	require.NoError(t, err)
	rel := filepath.Join("repos", owner, repoName, "config.db")
	_ = mustGit(t, repoDir, "add", rel)
	_ = mustGit(t, repoDir, "commit", "-m", "attention")
}

func clearMembers(t *testing.T, repoDir string) {
	t.Helper()
	configPath := filepath.Join(repoDir, "repos", owner, repoName, "config.db")
	db, err := configdb.OpenAndMigrate(t.Context(), configPath)
	require.NoError(t, err)
	statements := []string{
		`DELETE FROM github_check`,
		`DELETE FROM fixer_member`,
		`DELETE FROM reviewer_member`,
		`DELETE FROM builder_member`,
		`DELETE FROM member`,
	}
	for _, statement := range statements {
		_, err = db.DB.ExecContext(t.Context(), statement)
		require.NoError(t, err)
	}
	err = db.Close()
	require.NoError(t, err)
	rel := filepath.Join("repos", owner, repoName, "config.db")
	_ = mustGit(t, repoDir, "add", rel)
	_ = mustGit(t, repoDir, "commit", "-m", "clear members")
}

func runTick(t *testing.T, repoDir string) (id string) {
	t.Helper()
	cmd := tick.NewTick()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	t.Chdir(repoDir)

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)
	id = strings.TrimSpace(output.String())
	require.NotEmpty(t, id)
	return
}

func runCompile(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	cmd := compile.NewCompile()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	t.Chdir(repoDir)
	cmd.SetArgs([]string{"--tick", id})
	err = cmd.ExecuteContext(t.Context())
	if err == nil {
		assert.Empty(t, output.String())
	}
	return
}

func openStateTip(t *testing.T, repoDir string) (db *statedb.DB) {
	t.Helper()
	tipPath := filepath.Join(JunkDir, t.Name()+"-state-tip.db")
	spec := "state:repos/" + owner + "/" + repoName + "/state.db"
	blob := mustGit(t, repoDir, "show", spec)
	err := os.WriteFile(tipPath, blob, 0644)
	require.NoError(t, err)
	db, err = statedb.Open(t.Context(), tipPath)
	require.NoError(t, err)
	return
}

func listBuilders(t *testing.T, db *statedb.DB) (rows []records.ListBuilderMembersRow) {
	t.Helper()
	rows, err := db.Queries.ListBuilderMembers(t.Context())
	require.NoError(t, err)
	return
}

func listReviewers(t *testing.T, db *statedb.DB) (rows []records.ListReviewerMembersRow) {
	t.Helper()
	rows, err := db.Queries.ListReviewerMembers(t.Context())
	require.NoError(t, err)
	return
}

func listFixers(t *testing.T, db *statedb.DB) (rows []records.ListFixerMembersRow) {
	t.Helper()
	rows, err := db.Queries.ListFixerMembers(t.Context())
	require.NoError(t, err)
	return
}

func listChecks(t *testing.T, db *statedb.DB) (rows []records.GithubCheck) {
	t.Helper()
	rows, err := db.Queries.ListGithubChecks(t.Context())
	require.NoError(t, err)
	return
}
