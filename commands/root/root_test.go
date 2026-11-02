package root

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewRoot_should_list_bind(t *testing.T) {
	cmd := NewRoot()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--help"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "bind")
	assert.Contains(t, text, "Install the tick workflow and migrate config")
}

func Test_NewRoot_should_list_tick(t *testing.T) {
	cmd := NewRoot()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--help"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "tick")
	assert.Contains(t, text, "Record a tick and print its id")
}

func Test_NewRoot_should_list_label(t *testing.T) {
	cmd := NewRoot()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--help"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "label")
	assert.Contains(t, text, "Ensure configured labels exist")
}

func Test_NewRoot_should_list_setup(t *testing.T) {
	cmd := NewRoot()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--help"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "setup")
	assert.Contains(t, text, "Record config for app repos")
}

func Test_NewRoot_should_not_list_copy(t *testing.T) {
	cmd := NewRoot()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--help"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	text := output.String()
	assert.NotContains(t, text, "copy")
}
