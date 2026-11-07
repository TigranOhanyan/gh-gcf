package development

import (
	"github.com/spf13/cobra"
)

func NewSetupDevelopment() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "development",
		Short: "Add a development",
	}
	cmd.AddCommand(newAdd())
	cmd.AddCommand(newCheck())
	return
}
