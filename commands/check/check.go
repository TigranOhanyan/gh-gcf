package check

import (
	"github.com/spf13/cobra"
)

func NewCheck() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "check",
		Short: "Record a finished builder",
	}
	return
}
