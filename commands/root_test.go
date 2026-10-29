package commands

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewRoot_should_list_the_copy_command(t *testing.T) {
	cmd := NewRoot()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--help"})

	err := cmd.ExecuteContext(t.Context())
	require.NoError(t, err)

	text := output.String()
	assert.Contains(t, text, "copy")
	assert.Contains(t, text, "Copy config values into state")
}
