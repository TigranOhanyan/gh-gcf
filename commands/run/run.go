package run

import (
	"github.com/spf13/cobra"
)

func NewRun() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "run",
		Short: "Start a builder for a development",
	}
	return
}
