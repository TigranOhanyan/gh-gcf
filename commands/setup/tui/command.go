package tui

import (
	"os"

	"charm.land/log/v2"
	"github.com/spf13/cobra"
)

func NewTui() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "tui",
		Short: "Record config interactively",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			dir, err := os.Getwd()
			if err != nil {
				log.Error("current directory", "err", err)
				return
			}
			err = Run(cmd.Context(), dir)
			return
		},
	}
	return
}
