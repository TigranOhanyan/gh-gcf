package commands_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TigranOhanyan/gh-gcf/commands"
)

func Test_Cmd_should_run_a_quoted_argument(t *testing.T) {
	var got string
	ran := false
	root := &cobra.Command{Use: "gh-gcf"}
	check := &cobra.Command{Use: "check"}
	add := &cobra.Command{
		Use:  "add",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			ran = true
			got = cmd.Flag("name").Value.String()
			return
		},
	}
	add.Flags().String("name", "", "check name")
	check.AddCommand(add)
	root.AddCommand(check)
	cmd := commands.NewCmd(root)

	err := cmd.Shell(t.Context(), `check add --name "ci / unit"`)
	require.NoError(t, err)

	assert.True(t, ran)
	assert.Equal(t, "ci / unit", got)
}

func Test_Cmd_should_refuse_an_unterminated_quote(t *testing.T) {
	ran := false
	root := &cobra.Command{
		Use: "gh-gcf",
		RunE: func(*cobra.Command, []string) (err error) {
			ran = true
			return
		},
	}
	cmd := commands.NewCmd(root)

	err := cmd.Shell(t.Context(), `check add --name "ci`)
	require.Error(t, err)

	assert.False(t, ran)
}
