package compile

import (
	"github.com/spf13/cobra"
)

func NewCompile() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "compile",
		Short: "Copy a flow from config into state",
	}
	return
}
