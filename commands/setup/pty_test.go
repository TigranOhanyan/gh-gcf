package setup

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	form := session.mark()
	session.wait(form, "color:")
	session.write(" bug ")
	session.wait(form, "bug")
	session.write("\r")
	session.wait(form, "save")
	session.write("#EDEDED")
	session.wait(form, "EDEDED")
	startOfTest := time.Now().Add(-time.Second)
	session.write("\r")
	session.wait(form, "bug  ededed")
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

func Test_Setup_should_quit_from_the_repo_list(t *testing.T) {
	repoDir := setupCheckout(t)
	session := startSetup(t, repoDir)
	session.wait(0, "Repos")
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

	message := mustGit(t, repoDir, "log", "-1", "--format=%s")
	assert.Equal(t, "init", strings.TrimSpace(string(message)))

	tracked := mustGit(t, repoDir, "ls-files", "repos/octo/app/config.db")
	assert.Empty(t, strings.TrimSpace(string(tracked)))

	_, err = os.Stat(filepath.Join(repoDir, "repos", "octo", "app", "state.db"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Setup_should_write_config_on_the_checked_out_branch(t *testing.T) {
	repoDir := setupCheckout(t)
	_ = mustGit(t, repoDir, "checkout", "-b", "review")

	recordRepo(t, repoDir)

	branch := mustGit(t, repoDir, "branch", "--show-current")
	assert.Equal(t, "review", strings.TrimSpace(string(branch)))

	_, err := os.Stat(filepath.Join(repoDir, "repos", "octo", "app", "config.db"))
	require.NoError(t, err)

	message := mustGit(t, repoDir, "log", "-1", "--format=%s")
	assert.Equal(t, "init", strings.TrimSpace(string(message)))
}

func Test_Setup_should_refuse_an_existing_repo_folder(t *testing.T) {
	repoDir := setupCheckout(t)
	recordRepo(t, repoDir)

	session := startSetup(t, repoDir)
	session.wait(0, "octo/app")
	session.write("a")
	session.wait(0, "Add a repo")
	owner := session.mark()
	session.write("octo")
	session.wait(owner, "octo")
	session.write("\r")
	session.wait(owner, "save")
	session.write("app")
	session.write("\r")
	session.wait(owner, "already exists")
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
	form := session.mark()
	session.write("a")
	session.wait(form, "color:")
	session.write("bug")
	session.write("\r")
	session.wait(form, "save")
	session.write("ffffff")
	session.write("\r")
	session.wait(form, "already exists")
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
	form := session.mark()
	session.wait(form, "color:")
	session.write("bug")
	session.write("\r")
	session.wait(form, "save")
	session.write("red")
	session.write("\r")
	session.wait(form, "six hex digits")
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
	form := session.mark()
	session.wait(form, "color:")
	session.write(longName)
	session.write("\r")
	session.wait(form, "save")
	session.write("ededed")
	session.write("\r")
	session.wait(form, "longer than 50 characters")
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
	session.wait(0, "Repos")
	session.write("a")
	session.wait(0, "Add a repo")
	session.write("octo/app")
	session.write("\r")
	session.wait(0, "save")
	session.write("web")
	session.write("\r")
	session.wait(0, "single path segment")
	session.leave()
	err := session.quit()
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(repoDir, "repos"))
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func Test_Setup_should_refuse_an_empty_repo_name(t *testing.T) {
	repoDir := setupCheckout(t)
	session := startSetup(t, repoDir)
	session.wait(0, "Repos")
	session.write("a")
	session.wait(0, "Add a repo")
	session.write("octo")
	session.write("\r")
	session.wait(0, "save")
	session.write("\r")
	session.wait(0, "name is empty")
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
	session.wait(0, "octo/app")
	session.write("\r")
	session.wait(0, "config.db is missing")
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
	session.wait(0, "octo/app")
	session.write("\r")
	session.wait(0, "No labels yet.")
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
	form := session.mark()
	session.wait(form, "color:")
	session.write("bug")
	session.wait(form, "bug")
	session.escape()
	err := session.quit()
	require.NoError(t, err)

	labels := storedLabels(t, repoDir, "octo", "app")
	assert.Empty(t, labels)
}

func recordRepo(t *testing.T, repoDir string) {
	t.Helper()
	session := startSetup(t, repoDir)
	session.wait(0, "Repos")
	session.write("a")
	session.wait(0, "Add a repo")
	owner := session.mark()
	session.write("octo")
	session.wait(owner, "octo")
	session.write("\r")
	session.wait(owner, "save")
	name := session.mark()
	session.write("app")
	session.wait(name, "app")
	session.write("\r")
	session.wait(name, "No labels yet.")
	err := session.quit()
	require.NoError(t, err)
}

func selectRepo(t *testing.T, repoDir string) (session *terminalSession) {
	t.Helper()
	session = startSetup(t, repoDir)
	session.wait(0, "octo/app")
	session.write("\r")
	session.wait(0, "No labels yet.")
	return
}

func submitLabel(t *testing.T, session *terminalSession, name string, color string, row string) {
	t.Helper()
	session.write("a")
	form := session.mark()
	session.wait(form, "color:")
	session.write(name)
	session.wait(form, strings.TrimSpace(name))
	session.write("\r")
	session.wait(form, "save")
	session.write(color)
	session.write("\r")
	session.wait(form, row)
}

type terminalSession struct {
	t        *testing.T
	cmd      *exec.Cmd
	master   *os.File
	screen   termScreen
	closed   bool
	waitOnce sync.Once
	waitErr  error
}

func startSetup(t *testing.T, repoDir string) (session *terminalSession) {
	t.Helper()
	bin := setupBinary(t)
	cmd := exec.Command(bin, "setup")
	cmd.Dir = repoDir
	cmd.Env = withTerm(os.Environ())
	size := &pty.Winsize{Rows: 40, Cols: 120}
	master, err := pty.StartWithSize(cmd, size)
	require.NoError(t, err)

	session = &terminalSession{
		t:      t,
		cmd:    cmd,
		master: master,
	}
	go session.readScreen()
	t.Cleanup(func() {
		stopErr := session.stop()
		failed := stopErr != nil
		if failed {
			t.Errorf("stop setup: %v", stopErr)
		}
	})
	return
}

func (session *terminalSession) readScreen() {
	_, err := io.Copy(&session.screen, session.master)
	ended := err == nil || errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) || errors.Is(err, syscall.EIO)
	if ended {
		return
	}
	session.screen.setReadFailure(err)
}

func (session *terminalSession) write(text string) {
	session.t.Helper()
	payload := []byte(text)
	_, err := session.master.Write(payload)
	require.NoError(session.t, err)
}

func (session *terminalSession) wait(mark int, text string) {
	session.t.Helper()
	waitText(session.t, &session.screen, mark, text)
}

func (session *terminalSession) mark() (mark int) {
	mark = session.screen.mark()
	return
}

func (session *terminalSession) leave() {
	session.t.Helper()
	before := session.mark()
	session.escape()
	session.wait(before, "q quit")
}

func (session *terminalSession) escape() {
	session.write("\x1b")
	time.Sleep(150 * time.Millisecond)
}

func (session *terminalSession) quit() (err error) {
	session.write("q")
	err = session.waitFor(5 * time.Second)
	return
}

func (session *terminalSession) waitFor(limit time.Duration) (err error) {
	done := make(chan struct{})
	go func() {
		session.waitProcess()
		close(done)
	}()
	select {
	case <-done:
		err = session.waitErr
	case <-time.After(limit):
		err = fmt.Errorf("setup did not exit")
	}
	return
}

func (session *terminalSession) waitProcess() {
	session.waitOnce.Do(func() {
		session.waitErr = session.cmd.Wait()
	})
}

func (session *terminalSession) stop() (err error) {
	if session.closed {
		return
	}
	session.closed = true

	closeErr := session.master.Close()
	err = closeErr

	process := session.cmd.Process
	missing := process == nil
	if missing {
		err = joinErr(err, session.screen.readFailureErr())
		return
	}

	killErr := process.Kill()
	done := errors.Is(killErr, os.ErrProcessDone)
	failed := killErr != nil && !done
	if failed {
		err = joinErr(err, killErr)
	}
	session.waitProcess()
	signaled := isStopSignal(session.waitErr)
	unexpected := session.waitErr != nil && !signaled
	if unexpected {
		err = joinErr(err, session.waitErr)
	}
	err = joinErr(err, session.screen.readFailureErr())
	return
}

func isStopSignal(err error) (signaled bool) {
	var exitErr *exec.ExitError
	isExit := errors.As(err, &exitErr)
	if !isExit {
		return
	}
	signaled = exitErr.ProcessState.ExitCode() == -1
	return
}

func joinErr(left error, right error) (err error) {
	if left == nil {
		err = right
		return
	}
	if right == nil {
		err = left
		return
	}
	err = fmt.Errorf("%w; %v", left, right)
	return
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

func waitText(t *testing.T, screen *termScreen, mark int, text string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		found := screen.containsSince(mark, text)
		if found {
			return
		}
		expired := time.Now().After(deadline)
		if expired {
			t.Fatalf("timed out waiting for %q\n%s", text, screen.recent())
		}
		time.Sleep(20 * time.Millisecond)
	}
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

type termScreen struct {
	mu          sync.Mutex
	buf         bytes.Buffer
	readFailure error
}

func (screen *termScreen) Write(p []byte) (n int, err error) {
	screen.mu.Lock()
	defer screen.mu.Unlock()
	n, err = screen.buf.Write(p)
	return
}

func (screen *termScreen) setReadFailure(err error) {
	screen.mu.Lock()
	defer screen.mu.Unlock()
	screen.readFailure = err
}

func (screen *termScreen) readFailureErr() (err error) {
	screen.mu.Lock()
	defer screen.mu.Unlock()
	err = screen.readFailure
	return
}

func (screen *termScreen) mark() (mark int) {
	screen.mu.Lock()
	defer screen.mu.Unlock()
	mark = screen.buf.Len()
	return
}

func (screen *termScreen) containsSince(mark int, text string) (found bool) {
	screen.mu.Lock()
	defer screen.mu.Unlock()
	found = strings.Contains(screen.buf.String()[mark:], text)
	return
}

func (screen *termScreen) recent() (text string) {
	screen.mu.Lock()
	defer screen.mu.Unlock()
	text = screen.buf.String()
	const limit = 2000
	long := len(text) > limit
	if long {
		text = text[len(text)-limit:]
	}
	return
}
