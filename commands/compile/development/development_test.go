package development_test

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
	"github.com/TigranOhanyan/gh-gcf/statedb"
	"github.com/TigranOhanyan/gh-gcf/statedb/records"
)

const owner = "octo"
const repoName = "app"
const developmentName = "features"
const emptyInstructionHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func Test_CompileDevelopment_should_copy_a_development(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)

	err := runCompile(t, repoDir, id)
	require.NoError(t, err)

	subject := mustGit(t, repoDir, "log -1 --format=%s state")
	expectedSubject := fmt.Sprintf("✅ [%s] [compile_development] [octo/app]", id)
	assert.Equal(t, expectedSubject, strings.TrimSpace(string(subject)))
	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [features] recorded")

	db := openStateTip(t, repoDir)
	defer db.Close()
	rows := listDevelopments(t, db)
	require.Len(t, rows, 1)
	assert.Equal(t, developmentName, rows[0].Name)
	assert.Equal(t, "ready", rows[0].StartingLabel)
	assert.Equal(t, "stop", rows[0].AttentionLabel)
	assert.Equal(t, int64(42), rows[0].StandardsReviewerApp)
	assert.Equal(t, int64(43), rows[0].SpecificationReviewerApp)
	assert.Equal(t, emptyInstructionHash, rows[0].BuilderInstructionSha256)
	assert.Equal(t, emptyInstructionHash, rows[0].StandardsReviewerInstructionSha256)
	assert.Equal(t, emptyInstructionHash, rows[0].SpecificationReviewerInstructionSha256)
	assert.Equal(t, emptyInstructionHash, rows[0].FixerInstructionSha256)
}

func Test_CompileDevelopment_should_copy_a_development_without_writing_a_member(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)

	err := runCompile(t, repoDir, id)
	require.NoError(t, err)

	db := openStateTip(t, repoDir)
	defer db.Close()
	rows := listDevelopments(t, db)
	require.Len(t, rows, 1)
	assert.Equal(t, developmentName, rows[0].Name)
	assertAbsentTable(t, db.DB, "member")
}

func Test_CompileDevelopment_should_copy_a_check(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)

	err := runCompile(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Contains(t, string(body), "✅ [features] [ci / unit] recorded")

	db := openStateTip(t, repoDir)
	defer db.Close()
	checks := listChecks(t, db)
	require.Len(t, checks, 1)
	assert.Equal(t, developmentName, checks[0].DevelopmentName)
	assert.Equal(t, "ci / unit", checks[0].Name)
}

func Test_CompileDevelopment_should_store_a_hash_of_each_instruction(t *testing.T) {
	builder := "Build it."
	standards := "Hold the standard."
	specification := "Match the issue."
	fixer := "Apply the change."
	repoDir := setupFactory(t)
	writeInstructions(t, repoDir, builder, standards, specification, fixer)
	commitConfig(t, repoDir)
	id := runTick(t, repoDir)

	err := runCompile(t, repoDir, id)
	require.NoError(t, err)

	db := openStateTip(t, repoDir)
	defer db.Close()
	rows := listDevelopments(t, db)
	require.Len(t, rows, 1)
	assert.Equal(t, digest(builder), rows[0].BuilderInstructionSha256)
	assert.Equal(t, digest(standards), rows[0].StandardsReviewerInstructionSha256)
	assert.Equal(t, digest(specification), rows[0].SpecificationReviewerInstructionSha256)
	assert.Equal(t, digest(fixer), rows[0].FixerInstructionSha256)
}

func Test_CompileDevelopment_should_refuse_a_missing_instruction(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	rel := filepath.Join("repos", owner, repoName, "developments", developmentName, "fixer.md")
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
	rows := listDevelopments(t, db)
	assert.Empty(t, rows)
	checks := listChecks(t, db)
	assert.Empty(t, checks)
}

func Test_CompileDevelopment_should_leave_an_unchanged_development(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)

	err = runCompile(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Empty(t, strings.TrimSpace(string(body)))
	db := openStateTip(t, repoDir)
	defer db.Close()
	rows := listDevelopments(t, db)
	require.Len(t, rows, 1)
	assert.Equal(t, developmentName, rows[0].Name)
	checks := listChecks(t, db)
	require.Len(t, checks, 1)
	assert.Equal(t, "ci / unit", checks[0].Name)
}

func Test_CompileDevelopment_should_replace_a_changed_instruction_hash(t *testing.T) {
	repoDir := setupFactory(t)
	id := runTick(t, repoDir)
	err := runCompile(t, repoDir, id)
	require.NoError(t, err)
	text := "Build it."
	writeInstructions(t, repoDir, text, "", "", "")
	rel := filepath.Join("repos", owner, repoName, "developments", developmentName, "builder.md")
	addLine := fmt.Sprintf("add %s", rel)
	_ = mustGit(t, repoDir, addLine)
	_ = mustGit(t, repoDir, "commit -m instruction")

	err = runCompile(t, repoDir, id)
	require.NoError(t, err)

	body := mustGit(t, repoDir, "log -1 --format=%b state")
	assert.Equal(t, "✅ [features] updated", strings.TrimSpace(string(body)))
	db := openStateTip(t, repoDir)
	defer db.Close()
	rows := listDevelopments(t, db)
	require.Len(t, rows, 1)
	assert.Equal(t, digest(text), rows[0].BuilderInstructionSha256)
	assert.Equal(t, emptyInstructionHash, rows[0].StandardsReviewerInstructionSha256)
}

func Test_CompileDevelopment_should_require_a_tick_id(t *testing.T) {
	_, err := runRoot(t, "compile development")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tick")
}

func setupFactory(t *testing.T) (repoDir string) {
	t.Helper()
	repoDir = initFactory(t)
	mustRoot(t, "setup repo add --owner octo --name app")
	mustRoot(t, "setup label add --owner octo --repo app --name ready --color ededed")
	mustRoot(t, "setup label add --owner octo --repo app --name stop --color ff0000")
	mustRoot(t, "setup apps add --id 42")
	mustRoot(t, "setup apps attach --owner octo --repo app --id 42")
	mustRoot(t, "setup apps add --id 43")
	mustRoot(t, "setup apps attach --owner octo --repo app --id 43")
	line := fmt.Sprintf("setup development add --owner octo --repo app --name %s --starting %s --attention %s --standards-app %s --specification-app %s", developmentName, "ready", "stop", "42", "43")
	mustRoot(t, line)
	checkLine := fmt.Sprintf("setup development check add --owner octo --repo app --development %s --name %q", developmentName, "ci / unit")
	mustRoot(t, checkLine)
	writeInstructions(t, repoDir, "", "", "", "")
	commitConfig(t, repoDir)
	useStateBranch(t, repoDir)
	return
}

func writeInstructions(t *testing.T, repoDir string, builder string, standards string, specification string, fixer string) {
	t.Helper()
	writeInstruction(t, repoDir, "builder", builder)
	writeInstruction(t, repoDir, "standards_reviewer", standards)
	writeInstruction(t, repoDir, "specification_reviewer", specification)
	writeInstruction(t, repoDir, "fixer", fixer)
}

func writeInstruction(t *testing.T, repoDir string, role string, text string) {
	t.Helper()
	path := filepath.Join(repoDir, "repos", owner, repoName, "developments", developmentName, role+".md")
	err := os.MkdirAll(filepath.Dir(path), 0755)
	require.NoError(t, err)
	err = os.WriteFile(path, []byte(text), 0644)
	require.NoError(t, err)
}

func digest(text string) (sum string) {
	hashed := sha256.Sum256([]byte(text))
	sum = hex.EncodeToString(hashed[:])
	return
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
	line := fmt.Sprintf("compile development --tick %s", id)
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

func assertAbsentTable(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	var found string
	query := `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`
	err := db.QueryRowContext(t.Context(), query, name).Scan(&found)
	require.ErrorIs(t, err, sql.ErrNoRows)
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

func listDevelopments(t *testing.T, db *statedb.DB) (rows []records.Development) {
	t.Helper()
	rows, err := db.Queries.ListDevelopments(t.Context())
	require.NoError(t, err)
	return
}

func listChecks(t *testing.T, db *statedb.DB) (rows []records.DevelopmentCheck) {
	t.Helper()
	rows, err := db.Queries.ListDevelopmentChecks(t.Context())
	require.NoError(t, err)
	return
}
