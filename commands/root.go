package commands

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/bind"
)

func NewRoot() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "gh-gcf",
		Short: "Software factory",
	}
	bindCmd := bind.NewBind()
	cmd.AddCommand(bindCmd)
	return
}
