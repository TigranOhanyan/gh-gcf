package coding_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wiremock/go-wiremock"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

var JunkDir string
var wiremockClient = wiremock.NewClient("http://127.0.0.1:18443")

func TestMain(m *testing.M) {
	_, filename, _, _ := runtime.Caller(0)
	helperDir := filepath.Dir(filename)
	testDataDir := filepath.Join(helperDir, "testdata")
	JunkDir = filepath.Join(testDataDir, "junk")

	err := os.MkdirAll(JunkDir, 0755)
	if err != nil {
		os.Exit(1)
	}
	err = os.Setenv("GITHUB_API_URL", "http://127.0.0.1:18443/")
	if err != nil {
		os.Exit(1)
	}
	err = os.Setenv("CURSOR_API_URL", "http://127.0.0.1:18443")
	if err != nil {
		os.Exit(1)
	}
	err = os.Setenv("CURSOR_API_KEY", "cursor-test-key")
	if err != nil {
		os.Exit(1)
	}
	err = os.Unsetenv("FACTORY_GITHUB_APP_ID")
	if err != nil {
		os.Exit(1)
	}
	err = os.Unsetenv("FACTORY_GITHUB_APP_PRIVATE_KEY")
	if err != nil {
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func mustGit(t *testing.T, dir string, args ...string) (output []byte) {
	t.Helper()
	output, err := commands.Git(t.Context(), dir, args...)
	require.NoError(t, err)
	return
}
