package label

import (
	"github.com/spf13/cobra"
)

func NewSetupLabel() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "label",
		Short: "Add or list labels",
	}
	cmd.AddCommand(newAdd())
	cmd.AddCommand(newList())
	return
}
