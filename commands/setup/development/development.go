package development

import (
	"github.com/spf13/cobra"
)

func NewSetupDevelopment() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "development",
		Short: "Add or list developments",
	}
	cmd.AddCommand(newAdd())
	cmd.AddCommand(newList())
	cmd.AddCommand(newCheck())
	return
}
