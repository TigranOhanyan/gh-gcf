package apps

import (
	"github.com/spf13/cobra"
)

func NewSetupApps() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "apps",
		Short: "Add or list Apps",
	}
	cmd.AddCommand(newAdd())
	cmd.AddCommand(newList())
	cmd.AddCommand(newAttach())
	return
}
