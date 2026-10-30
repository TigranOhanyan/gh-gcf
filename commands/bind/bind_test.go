package bind

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	workflowfile "github.com/TigranOhanyan/gh-gcf/github"
	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/configdb"
)

func Test_Bind_should_commit_the_workflow_on_main(t *testing.T) {
	repoDir := setupMain(t)

	err := runBind(t, repoDir)
	require.NoError(t, err)

	message := mustGit(t, repoDir, "log", "-1", "--format=%s", "main")
	assert.Equal(t, commitMessage, strings.TrimSpace(string(message)))

	committed := mustGit(t, repoDir, "show", "main:"+workflowRel)
	assert.Equal(t, workflowfile.FactoryTick, committed)
}

func Test_Bind_should_migrate_config_and_commit_it_on_main(t *testing.T) {
	repoDir := setupMain(t)
	configRel := "repos/octo/app/config.db"
	writeEmptyDatabase(t, filepath.Join(repoDir, filepath.FromSlash(configRel)))
	_ = mustGit(t, repoDir, "add", configRel)
	_ = mustGit(t, repoDir, "commit", "-m", "config")

	err := runBind(t, repoDir)
	require.NoError(t, err)

	blob := mustGit(t, repoDir, "show", "main:"+configRel)
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
	err := runBind(t, repoDir)
	require.NoError(t, err)

	workflowPath := filepath.Join(repoDir, filepath.FromSlash(workflowRel))
	err = os.WriteFile(workflowPath, []byte("previous: true\n"), 0644)
	require.NoError(t, err)
	_ = mustGit(t, repoDir, "add", workflowRel)
	_ = mustGit(t, repoDir, "commit", "-m", "previous workflow")

	err = runBind(t, repoDir)
	require.NoError(t, err)

	committed := mustGit(t, repoDir, "show", "main:"+workflowRel)
	assert.Equal(t, workflowfile.FactoryTick, committed)
}

func Test_Bind_should_commit_the_worktrees_hint_on_main(t *testing.T) {
	repoDir := setupMain(t)

	err := runBind(t, repoDir)
	require.NoError(t, err)

	gitignore := mustGit(t, repoDir, "show", "main:"+gitignoreRel)
	assert.Equal(t, worktreesGitignore, gitignore)

	hint := mustGit(t, repoDir, "show", "main:"+hintRel)
	assert.Empty(t, hint)
}

func Test_Bind_should_refuse_a_directory_that_is_not_a_git_repo(t *testing.T) {
	repoDir := filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)

	err = runBind(t, repoDir)
	require.Error(t, err)

	_, err = os.Stat(filepath.Join(repoDir, filepath.FromSlash(workflowRel)))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	_, err = os.Stat(filepath.Join(repoDir, ".worktrees"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func runBind(t *testing.T, repoDir string) (err error) {
	t.Helper()
	cmd := NewBind()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	args := []string{"--factory-repo", repoDir}
	cmd.SetArgs(args)

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

	_ = mustGit(t, repoDir, "init", "-b", "main")
	_ = mustGit(t, repoDir, "config", "user.email", "test@example.com")
	_ = mustGit(t, repoDir, "config", "user.name", "test")
	_ = mustGit(t, repoDir, "commit", "--allow-empty", "-m", "init")
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
