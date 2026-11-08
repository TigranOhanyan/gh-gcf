package repo

import (
	"github.com/spf13/cobra"
)

func NewSetupRepo() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "repo",
		Short: "Add or list app repos",
	}
	cmd.AddCommand(newAdd())
	cmd.AddCommand(newList())
	return
}
