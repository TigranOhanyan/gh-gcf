package root

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/bind"
	"github.com/TigranOhanyan/gh-gcf/commands/tick"
)

func NewRoot() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "gh-gcf",
		Short: "Software factory",
	}
	bindCmd := bind.NewBind()
	cmd.AddCommand(bindCmd)
	tickCmd := tick.NewTick()
	cmd.AddCommand(tickCmd)
	return
}
