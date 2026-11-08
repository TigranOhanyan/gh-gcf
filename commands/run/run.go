package run

import (
	"github.com/spf13/cobra"
)

func NewRun() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "run",
		Short: "Start an agent for a development",
	}
	return
}
