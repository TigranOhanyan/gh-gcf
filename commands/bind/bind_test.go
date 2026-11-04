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
	t.Chdir(repoDir)

	err := runBind(t)
	require.NoError(t, err)

	message := mustGit(t, repoDir, "log", "-1", "--format=%s", "main")
	assert.Equal(t, commitMessage, strings.TrimSpace(string(message)))

	committed := mustGit(t, repoDir, "show", "main:"+workflowRel)
	expected := pinnedWorkflow()
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

	message := mustGit(t, repoDir, "log", "-1", "--format=%s", "main")
	assert.Equal(t, "init", strings.TrimSpace(string(message)))

	_, err = os.Stat(filepath.Join(repoDir, filepath.FromSlash(workflowRel)))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Bind_should_migrate_config_and_commit_it_on_main(t *testing.T) {
	repoDir := setupMain(t)
	configRel := "repos/octo/app/config.db"
	writeEmptyDatabase(t, filepath.Join(repoDir, filepath.FromSlash(configRel)))
	_ = mustGit(t, repoDir, "add", configRel)
	_ = mustGit(t, repoDir, "commit", "-m", "config")
	t.Chdir(repoDir)

	err := runBind(t)
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
	t.Chdir(repoDir)
	err := runBind(t)
	require.NoError(t, err)

	workflowPath := filepath.Join(repoDir, filepath.FromSlash(workflowRel))
	err = os.WriteFile(workflowPath, []byte("previous: true\n"), 0644)
	require.NoError(t, err)
	_ = mustGit(t, repoDir, "add", workflowRel)
	_ = mustGit(t, repoDir, "commit", "-m", "previous workflow")

	err = runBind(t)
	require.NoError(t, err)

	committed := mustGit(t, repoDir, "show", "main:"+workflowRel)
	expected := pinnedWorkflow()
	assert.Equal(t, expected, committed)
}

func Test_Bind_should_commit_the_worktrees_hint_on_main(t *testing.T) {
	repoDir := setupMain(t)
	t.Chdir(repoDir)

	err := runBind(t)
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

func pinnedWorkflow() (contents []byte) {
	token := []byte(versionToken)
	version := []byte(Version)
	contents = bytes.ReplaceAll(workflowfile.FactoryTick, token, version)
	marker := []byte(appKeyMarker)
	contents = bytes.ReplaceAll(contents, marker, nil)
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

func Test_Bind_should_name_each_registered_app_key(t *testing.T) {
	repoDir := setupMain(t)
	appsDir := filepath.Join(repoDir, "apps")
	err := os.MkdirAll(appsDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(appsDir, "42"), []byte{}, 0644)
	require.NoError(t, err)
	t.Chdir(repoDir)

	err = runBind(t)
	require.NoError(t, err)

	committed := mustGit(t, repoDir, "show", "main:"+workflowRel)
	text := string(committed)
	assert.Contains(t, text, "GITHUB_APP_42: ${{ secrets.GITHUB_APP_42 }}")
	assert.NotContains(t, text, "      __APP_KEYS__")
}

func Test_Bind_should_commit_the_workflow_on_the_open_branch(t *testing.T) {
	repoDir := setupMain(t)
	_ = mustGit(t, repoDir, "checkout", "-b", "bind-me")
	t.Chdir(repoDir)

	err := runBind(t)
	require.NoError(t, err)

	head := mustGit(t, repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	assert.Equal(t, "bind-me", strings.TrimSpace(string(head)))

	message := mustGit(t, repoDir, "log", "-1", "--format=%s")
	assert.Equal(t, commitMessage, strings.TrimSpace(string(message)))

	committed := mustGit(t, repoDir, "show", "HEAD:"+workflowRel)
	expected := pinnedWorkflow()
	assert.Equal(t, expected, committed)

	mainTip := mustGit(t, repoDir, "log", "-1", "--format=%s", "main")
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
