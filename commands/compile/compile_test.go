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

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
	"github.com/TigranOhanyan/gh-gcf/configdb"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const owner = "octo"
const repoName = "app"
const appID int64 = 123
const emptyInstructionHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

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

	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("✅ [%s] [compile_coding] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log -1 --format=%b state")
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
	assert.Equal(t, emptyInstructionHash, builders[0].InstructionSha256)

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

	body := mustGit(t, repoDir, "log -1 --format=%b state")
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

	body := mustGit(t, repoDir, "log -1 --format=%b state")
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

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Empty(t, strings.TrimSpace(string(body)))
	db := openStateTip(t, repoDir)
	defer db.Close()
	builders := listBuilders(t, db)
	require.Len(t, builders, 1)
	assert.Equal(t, "features", builders[0].Name)
	assert.Equal(t, "todo", builders[0].StartingLabel)
	assert.Equal(t, "attention", builders[0].AttentionLabel)
}

func Test_Compile_should_update_a_changed_instruction(t *testing.T) {
	spec := builderSpec{
		name:      "features",
		starting:  "todo",
		attention: "attention",
	}
	repoDir := setupFactory(t, spec)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	writeInstruction(t, repoDir, "features", "Build it.")
	rel := filepath.Join("repos", owner, repoName, "members", "features.md")
	addLine := fmt.Sprintf("add %s", rel)
	_ = mustGit(t, repoDir, addLine)
	_ = mustGit(t, repoDir, "commit -m instruction")

	err = runCompile(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Equal(t, "✅ [features] updated", strings.TrimSpace(string(body)))
	db := openStateTip(t, repoDir)
	defer db.Close()
	builders := listBuilders(t, db)
	require.Len(t, builders, 1)
	assert.NotEqual(t, emptyInstructionHash, builders[0].InstructionSha256)
}

func Test_Compile_should_fail_when_the_instruction_is_missing(t *testing.T) {
	spec := builderSpec{
		name:      "features",
		starting:  "todo",
		attention: "attention",
	}
	repoDir := setupFactory(t, spec)
	id := runTick(t, repoDir)
	rel := filepath.Join("repos", owner, repoName, "members", "features.md")
	err := os.Remove(filepath.Join(repoDir, rel))
	require.NoError(t, err)
	addLine := fmt.Sprintf("add %s", rel)
	_ = mustGit(t, repoDir, addLine)
	_ = mustGit(t, repoDir, `commit -m "remove instruction"`)

	err = runCompile(t, repoDir, id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")

	db := openStateTip(t, repoDir)
	defer db.Close()
	builders := listBuilders(t, db)
	assert.Empty(t, builders)
}

func Test_Compile_should_require_a_tick_id(t *testing.T) {
	_, err := runRoot(t, "compile coding")
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
	repoDir = initFactory(t)
	mustRoot(t, "setup repo add --owner octo --name app")
	names := []string{spec.starting, spec.attention}
	names = append(names, spec.extra...)
	if spec.hasReview {
		names = append(names, spec.reviewer.attention)
		text := fmt.Sprintf("%d", spec.reviewer.app)
		mustRoot(t, "setup apps add --id "+text)
		mustRoot(t, "setup apps attach --owner octo --repo app --id "+text)
	}
	if spec.hasFixer {
		names = append(names, spec.fixer.attention)
	}
	addLabels(t, names)
	builderLine := fmt.Sprintf("setup member add-builder --owner octo --repo app --name %q --starting %q --attention %q", spec.name, spec.starting, spec.attention)
	mustRoot(t, builderLine)
	if spec.hasReview {
		reviewLine := fmt.Sprintf("setup member add-reviewer --owner octo --repo app --builder %q --name %q --app %d --attention %q", spec.name, spec.reviewer.name, spec.reviewer.app, spec.reviewer.attention)
		mustRoot(t, reviewLine)
	}
	if spec.hasFixer {
		fixLine := fmt.Sprintf("setup member add-fixer --owner octo --repo app --builder %q --name %q --attention %q", spec.name, spec.fixer.name, spec.fixer.attention)
		mustRoot(t, fixLine)
	}
	if spec.check != "" {
		checkLine := fmt.Sprintf("setup check add --owner octo --repo app --builder %q --name %q", spec.name, spec.check)
		mustRoot(t, checkLine)
	}
	commitConfig(t, repoDir)
	useStateBranch(t, repoDir)
	return
}

func addLabels(t *testing.T, names []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, name := range names {
		already := seen[name]
		if already {
			continue
		}
		seen[name] = true
		line := fmt.Sprintf("setup label add --owner octo --repo app --name %q --color ededed", name)
		mustRoot(t, line)
	}
}

func initFactory(t *testing.T) (repoDir string) {
	t.Helper()
	repoDir = filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)
	_ = mustGit(t, repoDir, "init -b main")
	_ = mustGit(t, repoDir, "config user.email test@example.com")
	_ = mustGit(t, repoDir, "config user.name test")
	_ = mustGit(t, repoDir, "commit --allow-empty -m init")
	t.Chdir(repoDir)
	return
}

func commitConfig(t *testing.T, repoDir string) {
	t.Helper()
	_ = mustGit(t, repoDir, "add -A")
	_ = mustGit(t, repoDir, "commit -m config")
}

func useStateBranch(t *testing.T, repoDir string) {
	t.Helper()
	_ = mustGit(t, repoDir, "checkout -b state")
	_ = mustGit(t, repoDir, "checkout main")
}

func writeInstruction(t *testing.T, repoDir string, name string, text string) {
	t.Helper()
	path := filepath.Join(repoDir, "repos", owner, repoName, "members", name+".md")
	err := os.MkdirAll(filepath.Dir(path), 0755)
	require.NoError(t, err)
	err = os.WriteFile(path, []byte(text), 0644)
	require.NoError(t, err)
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
	addLine := fmt.Sprintf("add %s", rel)
	_ = mustGit(t, repoDir, addLine)
	_ = mustGit(t, repoDir, "commit -m attention")
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
	addLine := fmt.Sprintf("add %s", rel)
	_ = mustGit(t, repoDir, addLine)
	_ = mustGit(t, repoDir, `commit -m "clear members"`)
}

func runTick(t *testing.T, repoDir string) (id string) {
	t.Helper()
	t.Chdir(repoDir)
	stdout := mustRoot(t, "tick")
	id = strings.TrimSpace(stdout)
	require.NotEmpty(t, id)
	return
}

func runCompile(t *testing.T, repoDir string, id string) (err error) {
	t.Helper()
	t.Chdir(repoDir)
	line := fmt.Sprintf("compile coding --tick %s", id)
	stdout, err := runRoot(t, line)
	if err == nil {
		assert.Empty(t, stdout)
	}
	return
}

func runRoot(t *testing.T, line string) (stdout string, err error) {
	t.Helper()
	command := root.NewRoot()
	var out bytes.Buffer
	var errOut bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&errOut)
	wrapped := commands.NewCmd(command)
	err = wrapped.Shell(t.Context(), line)
	stdout = out.String()
	return
}

func mustRoot(t *testing.T, line string) (stdout string) {
	t.Helper()
	stdout, err := runRoot(t, line)
	require.NoError(t, err)
	return
}

func openStateTip(t *testing.T, repoDir string) (db *statedb.DB) {
	t.Helper()
	tipPath := filepath.Join(JunkDir, t.Name()+"-state-tip.db")
	spec := "state:repos/" + owner + "/" + repoName + "/state.db"
	showLine := fmt.Sprintf("show %s", spec)
	blob := mustGit(t, repoDir, showLine)
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
