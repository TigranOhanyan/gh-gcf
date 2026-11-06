package bind_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/root"
)

func Test_Bind_should_name_each_registered_app_key(t *testing.T) {
	repoDir := bindCheckout(t)
	t.Chdir(repoDir)
	mustRoot(t, "setup apps add --id 42")

	_, err := runRoot(t, "bind")
	require.NoError(t, err)

	committed := gitOut(t, repoDir, "show main:.github/workflows/factory-tick.yml")
	text := string(committed)
	indent := indentOf(t, text, "FACTORY_GITHUB_APP_PRIVATE_KEY:")
	want := indent + "GITHUB_APP_42: ${{ secrets.GITHUB_APP_42 }}\n"
	assert.Contains(t, text, want)
	assert.NotContains(t, text, "__APP_KEYS__\n")
}

func bindCheckout(t *testing.T) (repoDir string) {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	helperDir := filepath.Dir(filename)
	repoDir = filepath.Join(helperDir, "testdata", "junk", t.Name())
	err := os.RemoveAll(repoDir)
	require.NoError(t, err)
	err = os.MkdirAll(repoDir, 0755)
	require.NoError(t, err)
	gitOut(t, repoDir, "init -b main")
	gitOut(t, repoDir, "config user.email test@example.com")
	gitOut(t, repoDir, "config user.name test")
	gitOut(t, repoDir, "commit --allow-empty -m init")
	return
}

func gitOut(t *testing.T, dir string, line string) (output []byte) {
	t.Helper()
	output, err := commands.Git(t.Context(), dir, line)
	require.NoError(t, err)
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

func indentOf(t *testing.T, text string, key string) (indent string) {
	t.Helper()
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		match := strings.HasPrefix(trimmed, key)
		if match {
			lead := len(line) - len(trimmed)
			indent = line[:lead]
			return
		}
	}
	t.Fatalf("workflow has no %s", key)
	return
}
