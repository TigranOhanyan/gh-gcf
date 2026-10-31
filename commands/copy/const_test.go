package copy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

var JunkDir string

func TestMain(m *testing.M) {
	_, filename, _, _ := runtime.Caller(0)
	helperDir := filepath.Dir(filename)
	testDataDir := filepath.Join(helperDir, "testdata")
	JunkDir = filepath.Join(testDataDir, "junk")

	err := os.MkdirAll(JunkDir, 0755)
	if err != nil {
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func mustGit(t *testing.T, dir string, args ...string) (output []byte) {
	t.Helper()
	output, err := git(t.Context(), dir, args...)
	require.NoError(t, err)
	return
}

func git(ctx context.Context, dir string, args ...string) (output []byte, err error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err = cmd.Output()
	if err == nil {
		return
	}
	exitErr, isExit := err.(*exec.ExitError)
	if isExit {
		detail := string(exitErr.Stderr)
		err = fmt.Errorf("%w: %s", err, detail)
	}
	return
}
