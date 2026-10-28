package commands

import "github.com/spf13/cobra"

func NewRoot() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "gh-gcf",
		Short: "Software factory",
	}
	copyCmd := NewCopy()
	cmd.AddCommand(copyCmd)
	return
}
