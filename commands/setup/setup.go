package setup

import (
	"context"
	"os"

	"charm.land/bubbletea/v2"
	"charm.land/log/v2"
	"github.com/spf13/cobra"

	"github.com/TigranOhanyan/gh-gcf/commands"
	"github.com/TigranOhanyan/gh-gcf/commands/setup/development"
)

func NewSetup() (cmd *cobra.Command) {
	cmd = &cobra.Command{
		Use:   "setup",
		Short: "Record config for app repos",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			dir, err := os.Getwd()
			if err != nil {
				log.Error("current directory", "err", err)
				return
			}
			err = Setup(cmd.Context(), dir)
			return
		},
	}
	cmd.AddCommand(newSetupRepo())
	cmd.AddCommand(newSetupLabel())
	cmd.AddCommand(newSetupApps())
	cmd.AddCommand(newSetupMember())
	cmd.AddCommand(newSetupCheck())
	cmd.AddCommand(development.NewSetupDevelopment())
	return
}

// Setup records config in the factory checkout at dir.
// It fails when dir is not a git checkout. It does not commit.
func Setup(ctx context.Context, dir string) (err error) {
	err = commands.RequireCheckout(ctx, dir)
	if err != nil {
		log.Error("setup", "path", dir, "err", err)
		return
	}
	log.Info("setup", "path", dir)

	initial := newModel(ctx, dir)
	option := tea.WithContext(ctx)
	program := tea.NewProgram(initial, option)
	final, err := program.Run()
	finished, isModel := final.(model)
	if isModel {
		finished.closeDB()
	}
	if err != nil {
		log.Error("setup", "path", dir, "err", err)
	}
	return
}
