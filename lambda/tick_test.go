package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
)

var junkDir string

func TestMain(m *testing.M) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		os.Exit(1)
	}
	junkDir = filepath.Join(filepath.Dir(filename), "testdata", "junk")
	err := os.MkdirAll(junkDir, 0755)
	if err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func Test_Tick_should_push_a_tick_to_origin(t *testing.T) {
	repoDir, origin := setupFactory(t)
	t.Setenv("CURSOR_API_KEY", "test")
	t.Setenv("GH_TOKEN", "test")
	t.Setenv("FACTORY_GITHUB_APP_ID", "")
	t.Setenv("FACTORY_GITHUB_APP_PRIVATE_KEY", "")
	t.Chdir(repoDir)

	err := runTick(t.Context(), repoDir, "")
	require.NoError(t, err)

	message := gitOut(t, origin, "log --format=%s state")
	assert.Contains(t, string(message), "[tick]")
}

func setupFactory(t *testing.T) (repoDir string, origin string) {
	t.Helper()
	repoDir = filepath.Join(junkDir, t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)
	gitOut(t, repoDir, "init -b main")
	gitOut(t, repoDir, "config user.email test@example.com")
	gitOut(t, repoDir, "config user.name test")
	gitOut(t, repoDir, "commit --allow-empty -m init")
	t.Chdir(repoDir)
	mustRoot(t, "setup repo add --owner octo --name app")
	gitOut(t, repoDir, "add -A")
	gitOut(t, repoDir, "commit -m config")
	gitOut(t, repoDir, "checkout -b state")
	gitOut(t, repoDir, "checkout main")

	origin = filepath.Join(junkDir, t.Name()+"-origin")
	err = os.RemoveAll(origin)
	require.NoError(t, err)
	initLine := "init --bare " + strconv.Quote(origin)
	gitOut(t, repoDir, initLine)
	remote := fmt.Sprintf("remote add origin %s", strconv.Quote(origin))
	gitOut(t, repoDir, remote)
	gitOut(t, repoDir, "push origin main")
	gitOut(t, repoDir, "push origin state")
	return
}

func gitOut(t *testing.T, dir string, line string) (output []byte) {
	t.Helper()
	output, err := commands.Git(t.Context(), dir, line)
	require.NoError(t, err)
	return
}

func mustRoot(t *testing.T, line string) {
	t.Helper()
	command := root.NewRoot()
	var out strings.Builder
	command.SetOut(&out)
	command.SetErr(&out)
	wrapped := commands.NewCmd(command)
	err := wrapped.Shell(t.Context(), line)
	require.NoError(t, err)
}
