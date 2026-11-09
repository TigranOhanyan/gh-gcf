package bind

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/configdb"
	"github.com/TigranOhanyan/gh-gcf/github"
)

func Test_Bind_should_commit_the_workflow_on_main(t *testing.T) {
	repoDir := setupMain(t)
	t.Chdir(repoDir)

	err := runBind(t)
	require.NoError(t, err)

	message := mustGit(t, repoDir, "log -1 --format=%s main")
	assert.Equal(t, commitMessage, strings.TrimSpace(string(message)))

	showLine := fmt.Sprintf("show main:%s", workflowRel)
	committed := mustGit(t, repoDir, showLine)
	expected := pinnedWorkflow(t)
	assert.Equal(t, expected, committed)
}

func Test_Bind_should_refuse_a_dev_version(t *testing.T) {
	repoDir := setupMain(t)
	t.Chdir(repoDir)
	previous := Version
	Version = devVersion
	t.Cleanup(func() {
		Version = previous
	})

	err := runBind(t)
	require.Error(t, err)

	message := mustGit(t, repoDir, "log -1 --format=%s main")
	assert.Equal(t, "init", strings.TrimSpace(string(message)))

	_, err = os.Stat(filepath.Join(repoDir, filepath.FromSlash(workflowRel)))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Bind_should_migrate_config_and_commit_it_on_main(t *testing.T) {
	repoDir := setupMain(t)
	configRel := "repos/octo/app/config.db"
	writeEmptyDatabase(t, filepath.Join(repoDir, filepath.FromSlash(configRel)))
	addLine := fmt.Sprintf("add %s", configRel)
	_ = mustGit(t, repoDir, addLine)
	_ = mustGit(t, repoDir, "commit -m config")
	t.Chdir(repoDir)

	err := runBind(t)
	require.NoError(t, err)

	showLine := fmt.Sprintf("show main:%s", configRel)
	blob := mustGit(t, repoDir, showLine)
	tipPath := filepath.Join(JunkDir, t.Name()+"-config.db")
	err = os.WriteFile(tipPath, blob, 0644)
	require.NoError(t, err)

	db, err := configdb.OpenIfMigrated(t.Context(), tipPath)
	require.NoError(t, err)
	err = db.Close()
	require.NoError(t, err)
}

func Test_Bind_should_replace_the_previous_workflow(t *testing.T) {
	repoDir := setupMain(t)
	t.Chdir(repoDir)
	err := runBind(t)
	require.NoError(t, err)

	workflowPath := filepath.Join(repoDir, filepath.FromSlash(workflowRel))
	err = os.WriteFile(workflowPath, []byte("previous: true\n"), 0644)
	require.NoError(t, err)
	addLine := fmt.Sprintf("add %s", workflowRel)
	_ = mustGit(t, repoDir, addLine)
	_ = mustGit(t, repoDir, `commit -m "previous workflow"`)

	err = runBind(t)
	require.NoError(t, err)

	showLine := fmt.Sprintf("show main:%s", workflowRel)
	committed := mustGit(t, repoDir, showLine)
	expected := pinnedWorkflow(t)
	assert.Equal(t, expected, committed)
}

func Test_Bind_should_commit_the_worktrees_hint_on_main(t *testing.T) {
	repoDir := setupMain(t)
	t.Chdir(repoDir)

	err := runBind(t)
	require.NoError(t, err)

	gitignoreLine := fmt.Sprintf("show main:%s", gitignoreRel)
	gitignore := mustGit(t, repoDir, gitignoreLine)
	assert.Equal(t, worktreesGitignore, gitignore)

	hintLine := fmt.Sprintf("show main:%s", hintRel)
	hint := mustGit(t, repoDir, hintLine)
	assert.Empty(t, hint)
}

func Test_Bind_should_refuse_a_directory_that_is_not_a_git_repo(t *testing.T) {
	repoDir := filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)
	t.Chdir(repoDir)

	err = runBind(t)
	require.Error(t, err)

	_, err = os.Stat(filepath.Join(repoDir, filepath.FromSlash(workflowRel)))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(repoDir, ".worktrees"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func pinnedWorkflow(t *testing.T) (contents []byte) {
	t.Helper()
	template, err := workflowfile.ReadFactoryTick()
	require.NoError(t, err)
	token := []byte(versionToken)
	version := []byte(Version)
	replaced := bytes.ReplaceAll(template, token, version)
	contents, err = replaceAppKeys(replaced, nil)
	require.NoError(t, err)
	return
}

func Test_Bind_should_refuse_a_subdirectory_of_a_git_checkout(t *testing.T) {
	repoDir := setupMain(t)
	sub := filepath.Join(repoDir, "nested")
	err := os.MkdirAll(sub, 0755)
	require.NoError(t, err)
	t.Chdir(sub)

	err = runBind(t)
	require.Error(t, err)

	_, err = os.Stat(filepath.Join(repoDir, filepath.FromSlash(workflowRel)))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(repoDir, ".worktrees"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Bind_should_commit_the_workflow_on_the_open_branch(t *testing.T) {
	repoDir := setupMain(t)
	_ = mustGit(t, repoDir, "checkout -b bind-me")
	t.Chdir(repoDir)

	err := runBind(t)
	require.NoError(t, err)

	head := mustGit(t, repoDir, "rev-parse --abbrev-ref HEAD")
	assert.Equal(t, "bind-me", strings.TrimSpace(string(head)))

	message := mustGit(t, repoDir, "log -1 --format=%s")
	assert.Equal(t, commitMessage, strings.TrimSpace(string(message)))

	showLine := fmt.Sprintf("show HEAD:%s", workflowRel)
	committed := mustGit(t, repoDir, showLine)
	expected := pinnedWorkflow(t)
	assert.Equal(t, expected, committed)

	mainTip := mustGit(t, repoDir, "log -1 --format=%s main")
	assert.Equal(t, "init", strings.TrimSpace(string(mainTip)))
}

func runBind(t *testing.T) (err error) {
	t.Helper()
	cmd := NewBind()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err = cmd.ExecuteContext(t.Context())
	return
}

func setupMain(t *testing.T) (repoDir string) {
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
	return
}

func writeEmptyDatabase(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)

	sqlDB, err := driver.Open(path)
	require.NoError(t, err)
	err = sqlDB.Ping()
	require.NoError(t, err)
	err = sqlDB.Close()
	require.NoError(t, err)
}
