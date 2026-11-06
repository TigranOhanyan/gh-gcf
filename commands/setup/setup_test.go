package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/vttest"
	"github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/configdb"
	configrecords "github.com/TigranOhanyan/gh-gcf/configdb/records"
)

func Test_Setup_should_refuse_a_directory_that_is_not_a_git_checkout(t *testing.T) {
	repoDir := filepath.Join(JunkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)

	session := startSetup(t, repoDir)
	err = session.waitExit(setupExitLimit)
	require.Error(t, err)

	_, err = os.Stat(filepath.Join(repoDir, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Setup_should_refuse_a_subdirectory_of_a_git_checkout(t *testing.T) {
	repoDir := setupCheckout(t)
	sub := filepath.Join(repoDir, "nested")
	err := os.MkdirAll(sub, 0755)
	require.NoError(t, err)

	session := startSetup(t, sub)
	err = session.waitExit(setupExitLimit)
	require.Error(t, err)

	_, err = os.Stat(filepath.Join(sub, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Setup_should_create_an_empty_config_database(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
}

func Test_Setup_should_store_a_label(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)

	session := selectRepo(t, repoDir)
	session.write("a")
	session.wait("color:")
	session.write(" bug ")
	session.wait("bug")
	session.write("\r")
	session.wait("save")
	session.write("#EDEDED")
	session.wait("EDEDED")
	startOfTest := time.Now().Add(-time.Second)
	session.write("\r")
	session.wait("bug  ededed")
	startOfCheck := time.Now().Add(time.Second)
	err := session.quit()
	require.NoError(t, err)

	labels := storedLabels(t, repoDir, "octo", "app")
	require.Len(t, labels, 1)
	assert.Equal(t, "bug", labels[0].Name)
	assert.Equal(t, "ededed", labels[0].Color)
	registered, err := time.Parse(time.RFC3339Nano, labels[0].RegisteredAt)
	require.NoError(t, err)
	assert.WithinRange(t, registered, startOfTest, startOfCheck)
}

func Test_Setup_should_attach_a_registered_app(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)

	session := startSetup(t, repoDir)
	session.wait("repos")
	session.write("j")
	session.write("\r")
	session.wait("No apps yet.")
	session.write("a")
	session.wait("app id:")
	session.write("42")
	session.write("\r")
	session.wait("GITHUB_APP_42")
	session.escape()
	session.wait("repos")
	session.write("\r")
	session.wait("octo/app")
	session.write("\r")
	session.wait("labels")
	session.write("j")
	session.write("\r")
	session.wait("No apps yet.")
	session.write("a")
	session.wait("42")
	session.write("\r")
	session.wait("GITHUB_APP_42")
	err := session.quit()
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(repoDir, "apps", "42"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size())

	path := filepath.Join(repoDir, "repos", "octo", "app", "config.db")
	db, err := configdb.OpenIfMigrated(t.Context(), path)
	require.NoError(t, err)
	t.Cleanup(func() {
		closeErr := db.Close()
		failed := closeErr != nil
		if failed {
			t.Errorf("close config: %v", closeErr)
		}
	})
	ids, err := db.Queries.ListGithubApps(t.Context())
	require.NoError(t, err)
	require.Len(t, ids, 1)
	assert.Equal(t, int64(42), ids[0])
}

func Test_Setup_should_quit_from_the_repo_list(t *testing.T) {
	repoDir := setupCheckout(t)
	session := startSetup(t, repoDir)
	session.wait("repos")
	err := session.quit()
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(repoDir, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Setup_should_leave_the_commit_to_the_user(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)

	path := filepath.Join(repoDir, "repos", "octo", "app", "config.db")
	_, err := os.Stat(path)
	require.NoError(t, err)

	message := mustGit(t, repoDir, "log -1 --format=%s")
	assert.Equal(t, "init", strings.TrimSpace(string(message)))

	tracked := mustGit(t, repoDir, "ls-files repos/octo/app/config.db")
	assert.Empty(t, strings.TrimSpace(string(tracked)))

	_, err = os.Stat(filepath.Join(repoDir, "repos", "octo", "app", "state.db"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Setup_should_write_config_on_the_checked_out_branch(t *testing.T) {
	repoDir := setupCheckout(t)
	_ = mustGit(t, repoDir, "checkout -b review")

	recordRepo(t, repoDir)

	branch := mustGit(t, repoDir, "branch --show-current")
	assert.Equal(t, "review", strings.TrimSpace(string(branch)))

	_, err := os.Stat(filepath.Join(repoDir, "repos", "octo", "app", "config.db"))
	require.NoError(t, err)

	message := mustGit(t, repoDir, "log -1 --format=%s")
	assert.Equal(t, "init", strings.TrimSpace(string(message)))
}

func Test_Setup_should_refuse_an_existing_repo_folder(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)

	session := startSetup(t, repoDir)
	openRepoList(t, session)
	session.wait("octo/app")
	session.write("a")
	session.wait("repos>add")
	session.write("octo")
	session.wait("octo")
	session.write("\r")
	session.wait("save")
	session.write("app")
	session.write("\r")
	session.wait("already exists")
	session.leave()
	err := session.quit()
	require.NoError(t, err)

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
}

func Test_Setup_should_refuse_a_duplicate_label(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)

	session := selectRepo(t, repoDir)
	submitLabel(t, session, "bug", "ededed", "bug  ededed")
	session.write("a")
	session.wait("color:")
	session.write("bug")
	session.write("\r")
	session.wait("save")
	session.write("ffffff")
	session.write("\r")
	session.wait("already exists")
	session.leave()
	err := session.quit()
	require.NoError(t, err)

	labels := storedLabels(t, repoDir, "octo", "app")
	require.Len(t, labels, 1)
	assert.Equal(t, "bug", labels[0].Name)
	assert.Equal(t, "ededed", labels[0].Color)
}

func Test_Setup_should_refuse_a_color_that_is_not_hex(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)

	session := selectRepo(t, repoDir)
	session.write("a")
	session.wait("color:")
	session.write("bug")
	session.write("\r")
	session.wait("save")
	session.write("red")
	session.write("\r")
	session.wait("six hex digits")
	session.leave()
	err := session.quit()
	require.NoError(t, err)

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
}

func Test_Setup_should_refuse_a_label_name_longer_than_50_characters(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)

	session := selectRepo(t, repoDir)
	longName := strings.Repeat("a", 51)
	session.write("a")
	session.wait("color:")
	session.write(longName)
	session.write("\r")
	session.wait("save")
	session.write("ededed")
	session.write("\r")
	session.wait("longer than 50 characters")
	session.leave()
	err := session.quit()
	require.NoError(t, err)

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
}

func Test_Setup_should_store_a_label_name_of_50_characters(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)

	name := strings.Repeat("a", 50)
	row := name + "  ededed"
	session := selectRepo(t, repoDir)
	submitLabel(t, session, name, "ededed", row)
	err := session.quit()
	require.NoError(t, err)

	labels := storedLabels(t, repoDir, "octo", "app")
	require.Len(t, labels, 1)
	assert.Equal(t, name, labels[0].Name)
}

func Test_Setup_should_refuse_a_repo_name_that_is_not_a_single_path_segment(t *testing.T) {
	repoDir := setupCheckout(t)
	session := startSetup(t, repoDir)
	openRepoList(t, session)
	session.write("a")
	session.wait("repos>add")
	session.write("octo/app")
	session.write("\r")
	session.wait("save")
	session.write("web")
	session.write("\r")
	session.wait("single path segment")
	session.leave()
	err := session.quit()
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(repoDir, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Setup_should_store_a_repo_when_tab_moves_to_the_name(t *testing.T) {
	repoDir := setupCheckout(t)
	session := startSetup(t, repoDir)
	openRepoList(t, session)
	session.write("a")
	session.wait("repos>add")
	session.write("octo")
	session.wait("octo")
	session.write("\t")
	session.wait("save")
	session.write("app")
	session.wait("name: app")
	screen := session.screen()
	assert.NotContains(t, screen, "octoapp")
	session.write("\r")
	session.wait("octo/app")
	err := session.quit()
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(repoDir, "repos", "octo", "app", "config.db"))
	require.NoError(t, err)
}

func Test_Setup_should_refuse_an_empty_repo_name(t *testing.T) {
	repoDir := setupCheckout(t)
	session := startSetup(t, repoDir)
	openRepoList(t, session)
	session.write("a")
	session.wait("repos>add")
	session.write("octo")
	session.write("\r")
	session.wait("save")
	session.write("\r")
	session.wait("name is empty")
	session.leave()
	err := session.quit()
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(repoDir, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Setup_should_leave_a_missing_config_database_uncreated(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)
	path := filepath.Join(repoDir, "repos", "octo", "app", "config.db")
	err := os.Remove(path)
	require.NoError(t, err)

	session := startSetup(t, repoDir)
	openRepoList(t, session)
	session.wait("octo/app")
	session.write("\r")
	session.wait("config.db is missing")
	session.write("a")
	err = session.quit()
	require.NoError(t, err)

	_, err = os.Stat(path)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Setup_should_migrate_a_config_database_that_is_behind(t *testing.T) {
	repoDir := setupCheckout(t)
	dir := filepath.Join(repoDir, "repos", "octo", "app")
	err := os.MkdirAll(dir, 0755)
	require.NoError(t, err)
	writeEmptyDatabase(t, filepath.Join(dir, "config.db"))

	session := startSetup(t, repoDir)
	openRepoList(t, session)
	session.wait("octo/app")
	session.write("\r")
	session.write("\r")
	session.wait("No labels yet.")
	submitLabel(t, session, "bug", "ededed", "bug  ededed")
	err = session.quit()
	require.NoError(t, err)

	labels := storedLabels(t, repoDir, "octo", "app")
	require.Len(t, labels, 1)
	assert.Equal(t, "bug", labels[0].Name)
}

func Test_Setup_should_leave_the_database_unchanged_when_add_is_cancelled(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)

	session := selectRepo(t, repoDir)
	session.write("a")
	session.wait("color:")
	session.write("bug")
	session.wait("bug")
	session.escape()
	session.wait("No labels yet.")
	err := session.quit()
	require.NoError(t, err)

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
}

func setupCheckout(t *testing.T) (repoDir string) {
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

func storedLabels(t *testing.T, repoDir string, owner string, name string) (labels []configrecords.Label) {
	t.Helper()
	path := filepath.Join(repoDir, "repos", owner, name, "config.db")
	db, err := configdb.OpenIfMigrated(t.Context(), path)
	require.NoError(t, err)
	t.Cleanup(func() {
		closeErr := db.Close()
		failed := closeErr != nil
		if failed {
			t.Errorf("close config: %v", closeErr)
		}
	})
	labels, err = db.Queries.ListLabels(t.Context())
	require.NoError(t, err)
	return
}

func writeEmptyDatabase(t *testing.T, path string) {
	t.Helper()
	sqlDB, err := driver.Open(path)
	require.NoError(t, err)
	err = sqlDB.Ping()
	require.NoError(t, err)
	err = sqlDB.Close()
	require.NoError(t, err)
}

func recordRepo(t *testing.T, repoDir string) {
	t.Helper()
	session := startSetup(t, repoDir)
	openRepoList(t, session)
	session.write("a")
	session.wait("repos>add")
	session.write("octo")
	session.wait("octo")
	session.write("\r")
	session.wait("save")
	session.write("app")
	session.wait("app")
	session.write("\r")
	session.wait("octo/app")
	err := session.quit()
	require.NoError(t, err)
}

func openRepoList(t *testing.T, session *terminalSession) {
	t.Helper()
	session.wait("repos")
	session.write("\r")
	session.wait("a add")
}

func selectRepo(t *testing.T, repoDir string) (session *terminalSession) {
	t.Helper()
	session = startSetup(t, repoDir)
	openRepoList(t, session)
	session.wait("octo/app")
	session.write("\r")
	session.wait("labels")
	session.write("\r")
	session.wait("No labels yet.")
	return
}

func submitLabel(t *testing.T, session *terminalSession, name string, color string, row string) {
	t.Helper()
	session.write("a")
	session.wait("color:")
	session.write(name)
	session.wait(strings.TrimSpace(name))
	session.write("\r")
	session.wait("save")
	session.write(color)
	session.write("\r")
	session.wait(row)
}

const setupCols = 120
const setupRows = 40
const setupExitLimit = 5 * time.Second
const setupDrawLimit = 10 * time.Second

type terminalSession struct {
	t      *testing.T
	cmd    *exec.Cmd
	term   *vttest.Terminal
	closed bool
}

func startSetup(t *testing.T, repoDir string) (session *terminalSession) {
	t.Helper()
	bin := setupBinary(t)
	cmd := exec.Command(bin, "setup")
	cmd.Dir = repoDir
	cmd.Env = withTerm(os.Environ())
	term, err := vttest.NewTerminal(t, setupCols, setupRows)
	require.NoError(t, err)
	err = term.Start(cmd)
	require.NoError(t, err)

	session = &terminalSession{
		t:    t,
		cmd:  cmd,
		term: term,
	}
	t.Cleanup(func() {
		session.stop()
	})
	return
}

func (session *terminalSession) write(text string) {
	session.t.Helper()
	payload := []byte(text)
	_, err := session.term.Emulator.InputPipe().Write(payload)
	require.NoError(session.t, err)
}

func (session *terminalSession) wait(text string) {
	session.t.Helper()
	deadline := time.Now().Add(setupDrawLimit)
	for {
		screen := session.screen()
		found := strings.Contains(screen, text)
		if found {
			return
		}
		expired := time.Now().After(deadline)
		if expired {
			session.t.Fatalf("timed out waiting for %q\n%s", text, screen)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (session *terminalSession) screen() (text string) {
	raw := session.term.Emulator.Render()
	text = stripEscapes(raw)
	return
}

func stripEscapes(raw string) (text string) {
	var b strings.Builder
	i := 0
	for i < len(raw) {
		plain := raw[i] != 0x1b
		if plain {
			b.WriteByte(raw[i])
			i++
			continue
		}
		i = skipEscape(raw, i)
	}
	text = b.String()
	return
}

func skipEscape(raw string, i int) (next int) {
	next = i + 1
	if next >= len(raw) {
		return
	}
	csi := raw[next] == '['
	if csi {
		next++
		for next < len(raw) {
			ch := raw[next]
			next++
			done := ch >= 0x40 && ch <= 0x7e
			if done {
				return
			}
		}
		return
	}
	next++
	return
}

func (session *terminalSession) leave() {
	session.t.Helper()
	session.escape()
	session.wait("q quit")
}

func (session *terminalSession) escape() {
	session.write("\x1b")
}

func (session *terminalSession) quit() (err error) {
	session.write("q")
	err = session.waitExit(setupExitLimit)
	return
}

func (session *terminalSession) waitExit(limit time.Duration) (err error) {
	done := make(chan error, 1)
	go func() {
		done <- session.term.Wait(session.cmd)
	}()
	select {
	case err = <-done:
	case <-time.After(limit):
		err = fmt.Errorf("setup did not exit")
	}
	return
}

func (session *terminalSession) stop() {
	if session.closed {
		return
	}
	session.closed = true
	process := session.cmd.Process
	if process != nil {
		_ = process.Kill()
	}
	_ = session.term.Close()
}

var (
	setupBinOnce sync.Once
	setupBin     string
	setupBinErr  error
)

func setupBinary(t *testing.T) (path string) {
	t.Helper()
	setupBinOnce.Do(func() {
		root := moduleRoot()
		setupBin = filepath.Join(JunkDir, "gh-gcf")
		cmd := exec.Command("go", "build", "-o", setupBin, ".")
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil {
			setupBinErr = execError(err, output)
		}
	})
	require.NoError(t, setupBinErr)
	path = setupBin
	return
}

func moduleRoot() (root string) {
	_, filename, _, _ := runtime.Caller(0)
	setupDir := filepath.Dir(filename)
	commandsDir := filepath.Dir(setupDir)
	root = filepath.Dir(commandsDir)
	return
}

func withTerm(env []string) (next []string) {
	next = make([]string, 0, len(env)+1)
	for _, entry := range env {
		isTerm := strings.HasPrefix(entry, "TERM=")
		if isTerm {
			continue
		}
		next = append(next, entry)
	}
	next = append(next, "TERM=xterm-256color")
	return
}

func execError(err error, output []byte) (wrapped error) {
	wrapped = &buildError{err: err, output: output}
	return
}

type buildError struct {
	err    error
	output []byte
}

func (err *buildError) Error() (text string) {
	text = err.err.Error() + "\n" + string(err.output)
	return
}

func (err *buildError) Unwrap() (cause error) {
	cause = err.err
	return
}
