package statedb

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
