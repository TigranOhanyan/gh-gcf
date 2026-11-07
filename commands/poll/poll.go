package poll

import (
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands/poll/agent"
)

func NewPoll() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "poll",
		Short: "Check agents and works",
	}
	agentCmd := agent.NewAgent()
	cmd.AddCommand(agentCmd)
	return
}
